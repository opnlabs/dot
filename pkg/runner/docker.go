package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gosimple/slug"
	"github.com/rs/xid"
)

const (
	BUILD_DIR     = ".done"
	ARTIFACTS_DIR = ".artifacts"
	WORKING_DIR   = "/app"
)

type DockerRunnerOptions struct {
	ShowImagePull     bool
	Stdout            io.Writer
	Stderr            io.Writer
	MountDockerSocket bool
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
	dockerOptions    DockerRunnerOptions
	authConfig       string
}

func NewDockerRunner(name string, artifactManager artifacts.ArtifactManager, dockerOptions DockerRunnerOptions) *DockerRunner {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	jobName := slug.Make(fmt.Sprintf("%s-%s", name, xid.New().String()))

	if dockerOptions.Stdout == nil {
		dockerOptions.Stdout = os.Stdout
	}
	if dockerOptions.Stderr == nil {
		dockerOptions.Stdout = os.Stderr
	}

	return &DockerRunner{
		name:             jobName,
		workingDirectory: wd,
		artifactManager:  artifactManager,
		dockerOptions:    dockerOptions,
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

func (d *DockerRunner) WithCredentials(username, password string) *DockerRunner {
	authConfig := registry.AuthConfig{
		Username: username,
		Password: password,
	}

	jsonVal, err := json.Marshal(authConfig)
	if err != nil {
		log.Fatal("could not create auth config for docker authentication: ", err)
	}
	d.authConfig = base64.URLEncoding.EncodeToString(jsonVal)
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

	if err := d.pullImage(ctx, cli); err != nil {
		return fmt.Errorf("could not pull image for container %s: %v", d.name, err)
	}

	commandScript := strings.Join(d.cmd, "\n")
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      d.image,
		Env:        d.env,
		Cmd:        []string{"/bin/sh", "-c", commandScript, ">/dev/null 2>&1"},
		WorkingDir: WORKING_DIR,
	}, &container.HostConfig{
		Mounts: d.prepareMounts(),
	}, nil, nil, d.name)
	if err != nil {
		return fmt.Errorf("unable to create container %s: %v", d.name, err)
	}
	d.containerID = resp.ID
	defer cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})

	if err := d.createSrcDirectories(ctx, cli); err != nil {
		return fmt.Errorf("unable to create source directories for %s: %v", d.name, err)
	}

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

	if _, err := stdcopy.StdCopy(d.dockerOptions.Stdout, d.dockerOptions.Stderr, logs); err != nil {
		return fmt.Errorf("unable to read container logs from %s: %v", d.name, err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container %s to stop: %v", d.name, err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with status code %d", status.StatusCode)
		}
		if err := d.publishArtifacts(); err != nil {
			return fmt.Errorf("unable to publish artifacts fpr %s: %v", d.name, err)
		}
	case <-ctx.Done():
		return fmt.Errorf("context timed out, stopping container %s", d.name)
	}

	return nil
}

func (d *DockerRunner) createSrcDirectories(ctx context.Context, cli *client.Client) error {
	f, err := os.CreateTemp("", "tarcopy-*.tar")
	if err != nil {
		return err
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := utils.CompressTar(d.src, f.Name()); err != nil {
		return err
	}

	tar, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	return cli.CopyToContainer(ctx, d.containerID, WORKING_DIR, tar, types.CopyToContainerOptions{})
}

func (d *DockerRunner) publishArtifacts() error {
	for _, v := range d.artifacts {
		if _, err := d.artifactManager.PublishArtifact(d.containerID, filepath.Join(WORKING_DIR, v)); err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerRunner) prepareMounts() []mount.Mount {
	var mounts []mount.Mount
	if d.dockerOptions.MountDockerSocket {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: "/var/run/docker.sock",
			Target: "/var/run/docker.sock",
		})
	}
	return mounts
}

func (d *DockerRunner) pullImage(ctx context.Context, cli *client.Client) error {
	reader, err := cli.ImagePull(ctx, d.image, types.ImagePullOptions{RegistryAuth: d.authConfig})
	if err != nil {
		return err
	}
	defer reader.Close()

	imageLogs := io.Discard
	if d.dockerOptions.ShowImagePull {
		imageLogs = d.dockerOptions.Stdout
	}
	if _, err := io.Copy(imageLogs, reader); err != nil {
		return err
	}

	return nil
}
