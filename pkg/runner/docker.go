package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/done/pkg/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/hashicorp/go-getter"
)

const ARTIFACT_DIR = ".done"

type DockerRunner struct {
	name             string
	image            string
	src              string
	env              []string
	cmd              []string
	containerID      string
	workingDirectory string
}

func NewDockerRunner(name string) *DockerRunner {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	jobName := slug.Make(name + uuid.NewString())
	return &DockerRunner{name: jobName, workingDirectory: wd}
}

func (d *DockerRunner) WithImage(image string) *DockerRunner {
	d.image = image
	return d
}

func (d *DockerRunner) WithSrc(src string) *DockerRunner {
	if src == "" {
		src = d.workingDirectory
	}
	d.src = src
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

func (d *DockerRunner) Run(output io.Writer) error {
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
	io.Copy(output, reader)

	d.createSrcDirectories()

	commandScript := strings.Join(d.cmd, "; ")
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      d.image,
		Env:        d.env,
		Cmd:        []string{"/bin/sh", "-c", commandScript},
		WorkingDir: "/app",
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: filepath.Join(d.workingDirectory, ARTIFACT_DIR, fmt.Sprintf("src-%s", d.name)),
				Target: "/app",
			},
		},
	}, nil, nil, d.name)
	if err != nil {
		log.Fatal(err)
	}
	d.containerID = resp.ID
	defer cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})

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
	io.Copy(output, logs)

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-statusCh:
	}

	return nil
}

func (d *DockerRunner) createSrcDirectories() error {
	client := &getter.Client{
		Ctx:  context.Background(),
		Dst:  filepath.Join(d.workingDirectory, ARTIFACT_DIR, fmt.Sprintf("src-%s", d.name)),
		Dir:  true,
		Src:  d.src,
		Pwd:  d.workingDirectory,
		Mode: getter.ClientModeDir,
		Detectors: []getter.Detector{
			&getter.FileDetector{},
			&getter.GitDetector{},
			&getter.GitHubDetector{},
			&getter.GitLabDetector{},
		},
		Getters: map[string]getter.Getter{
			"file": &getter.FileGetter{
				Copy: true,
			},
		},
	}

	if err := client.Get(); err != nil {
		log.Fatal(err)
	}

	return nil
}
