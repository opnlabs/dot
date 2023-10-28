package artifacts

import (
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/done/pkg/store"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const ARTIFACTS_DIR = ".artifacts"

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
}

func NewDockerArtifactsManager() ArtifactManager {
	// Clear previous artifacts and create a new directory
	if _, err := os.Stat(ARTIFACTS_DIR); err == nil {
		os.RemoveAll(ARTIFACTS_DIR)
	}
	os.Mkdir(ARTIFACTS_DIR, 0755)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	return &DockerArtifactsManager{
		cli:           cli,
		artifactStore: store.NewMemStore(),
	}
}

func (d *DockerArtifactsManager) PublishArtifact(jobID, path string) (string, error) {
	ctx := context.Background()
	r, _, err := d.cli.CopyFromContainer(ctx, jobID, path)
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp(ARTIFACTS_DIR, "artifacts-*.tar")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, r)
	if err != nil {
		return "", err
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
				return err
			}
			f, err := os.Open(v)
			if err != nil {
				log.Println(err)
				return err
			}
			defer f.Close()

			err = d.cli.CopyToContainer(ctx, jobID, originalPath.(string), f, types.CopyToContainerOptions{})
			if err != nil {
				return err
			}
		}
	}

	return filepath.Walk(ARTIFACTS_DIR, func(path string, info fs.FileInfo, err error) error {
		if !strings.Contains(path, ".tar") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			log.Println(err)
			return err
		}
		defer f.Close()

		_, fname := filepath.Split(path)
		log.Println("Retrieve name: ", fname)
		originalPath, err := d.artifactStore.Get(strings.TrimSpace(fname))
		if err != nil {
			log.Fatal(err)
		}

		return d.cli.CopyToContainer(context.Background(), jobID, originalPath.(string), f, types.CopyToContainerOptions{})
	})
}
