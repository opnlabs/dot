package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/done/pkg/artifacts"
	"github.com/cvhariharan/done/pkg/models"
	"github.com/cvhariharan/done/pkg/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
)

const (
	BUILD_DIR     = ".done"
	ARTIFACTS_DIR = ".artifacts"
	WORKING_DIR   = "/app"
)

type LogOptions struct {
	ShowImagePull bool
	Stdout        io.Writer
	Stderr        io.Writer
}

type DockerRunner struct {
	name             string
	image            string
	src              string
	env              []string
	cmd              []string
	containerID      string
	workingDirectory string
	artifacts        []string
	artifactManager  artifacts.ArtifactManager
	logOptions       LogOptions
}

func NewDockerRunner(name string, artifactManager artifacts.ArtifactManager, logOptions LogOptions) *DockerRunner {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	jobName := slug.Make(name + uuid.NewString())

	if logOptions.Stdout == nil {
		logOptions.Stdout = os.Stdout
	}
	if logOptions.Stderr == nil {
		logOptions.Stdout = os.Stderr
	}

	return &DockerRunner{
		name:             jobName,
		workingDirectory: wd,
		artifactManager:  artifactManager,
		logOptions:       logOptions,
	}
}

func (d *DockerRunner) WithImage(image string) *DockerRunner {
	d.image = image
	return d
}

func (d *DockerRunner) WithSrc(src string) *DockerRunner {
	d.src = filepath.Clean(src)
	return d
}

func (d *DockerRunner) WithEnv(env []models.Variable) *DockerRunner {
	variables := make([]string, 0)
	for _, v := range env {
		// TODO - Move this to a validation function
		if len(v) > 1 {
			log.Fatal("variables should be defined as a key value pair")
		}
		for k, v := range v {
			variables = append(variables, fmt.Sprintf("%s=%s", k, v))
		}
	}
	d.env = variables
	return d
}

func (d *DockerRunner) WithCmd(cmd []string) *DockerRunner {
	d.cmd = cmd
	return d
}

func (d *DockerRunner) CreatesArtifacts(artifacts []string) *DockerRunner {
	d.artifacts = artifacts
	return d
}

func (d *DockerRunner) Run(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("unable to create docker client to create container %s: %v", d.name, err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, d.image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("unable to pull image to create container %s: %v", d.name, err)
	}
	defer reader.Close()
	if d.logOptions.ShowImagePull {
		if _, err := io.Copy(d.logOptions.Stdout, reader); err != nil {
			return fmt.Errorf("unable to read image pull logs for %s: %v", d.name, err)
		}
	}

	if err := d.createSrcDirectories(cli); err != nil {
		return fmt.Errorf("unable to create source directories for %s: %v", d.name, err)
	}

	commandScript := strings.Join(d.cmd, "\n")
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      d.image,
		Env:        d.env,
		Cmd:        []string{"/bin/sh", "-c", commandScript},
		WorkingDir: WORKING_DIR,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: filepath.Join(d.workingDirectory, BUILD_DIR, fmt.Sprintf("src-%s", d.name)),
				Target: WORKING_DIR,
			},
		},
	}, nil, nil, d.name)
	if err != nil {
		return fmt.Errorf("unable to create container %s: %v", d.name, err)
	}
	d.containerID = resp.ID
	defer cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})

	if err := d.artifactManager.RetrieveArtifact(d.containerID, nil); err != nil {
		return fmt.Errorf("unable to retrieve artifacts for %s: %v", d.name, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("unable to start container %s: %v", d.name, err)
	}

	logs, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("unable to attach logs for %s: %v", d.name, err)
	}
	defer logs.Close()

	if _, err := io.Copy(d.logOptions.Stdout, logs); err != nil {
		return fmt.Errorf("unable to read container logs from %s: %v", d.name, err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container %s to stop: %v", d.name, err)
	case <-statusCh:
		if err := d.publishArtifacts(); err != nil {
			return fmt.Errorf("unable to publish artifacts fpr %s: %v", d.name, err)
		}
	case <-ctx.Done():
		return fmt.Errorf("context timed out, stopping container %s", d.name)
	}

	return nil
}

func (d *DockerRunner) createSrcDirectories(cli *client.Client) error {
	return utils.TarCopy(d.src, filepath.Join(BUILD_DIR, fmt.Sprintf("src-%s", d.name)), "")
}

func (d *DockerRunner) publishArtifacts() error {
	for _, v := range d.artifacts {
		if _, err := d.artifactManager.PublishArtifact(d.containerID, filepath.Join(WORKING_DIR, v)); err != nil {
			return err
		}
	}
	return nil
}
