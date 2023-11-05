package artifacts

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/dot/pkg/store"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type ArtifactManager interface {
	// PublishArtifact takes in a jobID and path inside the job and
	// moves the artifact to the artifact stores and returns a key
	// that references the artifact
	PublishArtifact(jobID, path string) (key string, err error)

	// RetrieveArtifact takes in a jobID, keys slice and
	// moves the artifact to the original path inside the job. If the keys is nil, all artifacts will be
	// moved into the job. The original path is the path from where the artifact was pushed in
	// PublishArtifact
	RetrieveArtifact(jobID string, keys []string) error
}

type DockerArtifactsManager struct {
	cli           *client.Client
	artifactStore store.Store
	artifactsDir  string
}

func NewDockerArtifactsManager(artifactsDir string) ArtifactManager {
	// Clear previous artifacts and create a new directory
	if _, err := os.Stat(artifactsDir); err == nil {
		if err := os.RemoveAll(artifactsDir); err != nil {
			log.Fatalf("could not remove %s directory: %v", artifactsDir, err)
		}
	}

	if err := os.Mkdir(artifactsDir, 0755); err != nil {
		log.Fatalf("could not create %s directory: %v", artifactsDir, err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	return &DockerArtifactsManager{
		cli:           cli,
		artifactStore: store.NewMemStore(),
		artifactsDir:  artifactsDir,
	}
}

func (d *DockerArtifactsManager) PublishArtifact(jobID, path string) (string, error) {
	ctx := context.Background()
	r, _, err := d.cli.CopyFromContainer(ctx, jobID, path)
	if err != nil {
		return "", fmt.Errorf("could not copy artifact %s from container %s: %v", path, jobID, err)
	}

	f, err := os.CreateTemp(d.artifactsDir, "artifacts-*.tar")
	if err != nil {
		return "", fmt.Errorf("could not create artifacts tar file: %v", err)
	}

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("could not copy file contents from container %s to artifact tar: %v", jobID, err)
	}

	_, fname := filepath.Split(f.Name())
	return fname, d.artifactStore.Set(strings.TrimSpace(fname), filepath.Dir(path))
}

func (d *DockerArtifactsManager) RetrieveArtifact(jobID string, keys []string) error {
	ctx := context.Background()

	if len(keys) > 0 {
		for _, v := range keys {
			originalPath, err := d.artifactStore.Get(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("could not find original path for artifact %s: %v", v, err)
			}
			f, err := os.Open(filepath.Clean(v))
			if err != nil {
				return fmt.Errorf("could not open artifact %s: %v", v, err)
			}
			defer f.Close()

			if err := d.cli.CopyToContainer(ctx, jobID, originalPath.(string), f, types.CopyToContainerOptions{}); err != nil {
				return fmt.Errorf("could not copy artifact %s to container %s: %v", v, jobID, err)
			}
		}
	}

	return filepath.Walk(d.artifactsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.Contains(path, ".tar") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("could not open %s artifact for copying to container %s: %v", path, jobID, err)
		}
		defer f.Close()

		_, fname := filepath.Split(path)
		originalPath, err := d.artifactStore.Get(strings.TrimSpace(fname))
		if err != nil {
			return fmt.Errorf("could not get %s from artifact store: %v", fname, err)
		}

		return d.cli.CopyToContainer(context.Background(), jobID, originalPath.(string), f, types.CopyToContainerOptions{})
	})
}
