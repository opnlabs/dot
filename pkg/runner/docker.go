package runner

import (
	"context"
	"io"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerRunner struct {
	name  string
	image string
	env   []string
	cmd   []string
}

func NewDockerRunner(name string) DockerRunner {
	return DockerRunner{name: name}
}

func (d DockerRunner) WithImage(image string) DockerRunner {
	d.image = image
	return d
}

func (d DockerRunner) WithEnv(env []string) DockerRunner {
	d.env = env
	return d
}

func (d DockerRunner) WithCmd(cmd []string) DockerRunner {
	d.cmd = cmd
	return d
}

func (d DockerRunner) Run(output io.Writer) error {
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

	commandScript := strings.Join(d.cmd, "; ")
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      d.image,
		Env:        d.env,
		Cmd:        []string{"/bin/sh", "-c", commandScript},
		WorkingDir: "/app",
	}, nil, nil, nil, d.name)
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
	io.Copy(output, logs)

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-statusCh:
		cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
	}

	return nil
}
