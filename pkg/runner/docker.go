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
	"github.com/cvhariharan/done/pkg/store"
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
	artifactStore    store.Store
	artifactManager  artifacts.ArtifactManager
}

func NewDockerRunner(name string, artifactManager artifacts.ArtifactManager) *DockerRunner {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	jobName := slug.Make(name + uuid.NewString())
	return &DockerRunner{
		name:             jobName,
		workingDirectory: wd,
		artifactStore:    store.NewMemStore(),
		artifactManager:  artifactManager,
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

func (d *DockerRunner) CreatesArtifacts(artifacts []string) *DockerRunner {
	d.artifacts = artifacts
	return d
}

func (d *DockerRunner) Run(logOptions LogOptions) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, d.image, types.ImagePullOptions{})
	if err != nil {
		log.Fatal(err)
	}
	if logOptions.ShowImagePull {
		io.Copy(logOptions.Stdout, reader)
	}

	err = d.createSrcDirectories(cli)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}
	d.containerID = resp.ID
	defer cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})

	err = d.artifactManager.RetrieveArtifact(d.containerID, nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatal(err)
	}

	logs, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(logOptions.Stdout, logs)

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-statusCh:
	}

	for _, v := range d.artifacts {
		_, err = d.artifactManager.PublishArtifact(d.containerID, filepath.Join(WORKING_DIR, v))
		if err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func (d *DockerRunner) createSrcDirectories(cli *client.Client) error {
	return utils.TarCopy(d.src, filepath.Join(BUILD_DIR, fmt.Sprintf("src-%s", d.name)), "")
}
