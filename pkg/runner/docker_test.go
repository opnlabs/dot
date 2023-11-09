package runner

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/opnlabs/dot/pkg/artifacts"
	"github.com/opnlabs/dot/pkg/models"
	"github.com/opnlabs/dot/pkg/utils"
	"github.com/stretchr/testify/assert"
)

type Test struct {
	Name        string
	Manager     artifacts.ArtifactManager
	Image       string
	Src         string
	Entrypoint  []string
	Script      []string
	Variables   []models.Variable
	Artifacts   []string
	Output      io.Writer
	Ctx         context.Context
	Expectation func(*testing.T, *bytes.Buffer) bool
	Username    string
	Password    string
}

func teardown(tb testing.TB) {

	wd, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return
	}
	os.RemoveAll(filepath.Join(wd, ".artifacts"))
}

func TestRun(t *testing.T) {

	var b bytes.Buffer
	ctx := context.Background()
	manager := artifacts.NewDockerArtifactsManager(".artifacts")

	// ctxTimeout, cancel := context.WithTimeout(ctx, time.Millisecond)
	// defer cancel()

	tests := []Test{
		{
			Name:    "Test Image",
			Manager: manager,
			Image:   "docker.io/alpine",
			Script: []string{
				"cat /etc/os-release",
			},
			Output:      &b,
			Expectation: testImageOutput,
			Ctx:         ctx,
			Username:    "",
			Password:    "",
		},
		{
			Name:    "Test Variables",
			Manager: manager,
			Image:   "docker.io/alpine",
			Variables: []models.Variable{
				map[string]string{
					"TESTING_VARIABLE": "TESTING",
				},
			},
			Script: []string{
				"echo $TESTING_VARIABLE",
			},
			Output:      &b,
			Expectation: testVariableOutput,
			Ctx:         ctx,
		},
		{
			Name:    "Test Create Artifact",
			Manager: manager,
			Image:   "docker.io/alpine",
			Script: []string{
				"echo TESTING >> log.txt",
			},
			Output: &b,
			Artifacts: []string{
				"log.txt",
			},
			Expectation: testArtifactCreation,
			Ctx:         ctx,
		},
		{
			Name:    "Test Retrieve Artifact",
			Manager: manager,
			Image:   "docker.io/alpine",
			Script: []string{
				"cat log.txt",
			},
			Output:      &b,
			Expectation: testVariableOutput,
			Ctx:         ctx,
		},
		{
			Name:       "Test Entrypoint",
			Manager:    manager,
			Image:      "docker.io/alpine",
			Entrypoint: []string{"echo"},
			Script: []string{
				"TESTING",
			},
			Output:      &b,
			Expectation: testVariableOutput,
			Ctx:         ctx,
		},
	}

	for _, test := range tests {
		b.Truncate(0)
		err := NewDockerRunner(test.Name, test.Manager, DockerRunnerOptions{ShowImagePull: false, Stdout: test.Output, Stderr: os.Stderr}).
			WithImage(test.Image).
			WithSrc(test.Src).
			WithEntrypoint(test.Entrypoint).
			WithCmd(test.Script).
			WithEnv(test.Variables).
			WithCredentials(test.Username, test.Password).
			CreatesArtifacts(test.Artifacts).Run(test.Ctx)
		assert.NoError(t, err, "error is nil")
		assert.Equal(t, true, test.Expectation(t, &b))
	}

	teardown(t)
}

func TestMountDockerSocket(t *testing.T) {
	manager := artifacts.NewDockerArtifactsManager(".artifacts")
	err := NewDockerRunner("Mount Docker Socket Test", manager, DockerRunnerOptions{MountDockerSocket: true, ShowImagePull: false, Stdout: nil, Stderr: nil}).
		WithImage("docker.io/alpine").
		Run(context.Background())
	if err != nil {
		t.Error(err)
	}
	teardown(t)
}

func TestNonExistingSrcDirectory(t *testing.T) {
	manager := artifacts.NewDockerArtifactsManager(".artifacts")
	err := NewDockerRunner("Non existing src directory", manager, DockerRunnerOptions{ShowImagePull: false, Stdout: nil, Stderr: nil}).
		WithImage("docker.io/alpine").
		WithSrc("testnonexisting").
		Run(context.Background())
	assert.ErrorContains(t, err, "unable to create source directories")
	teardown(t)
}

func TestPublishNonExistingFile(t *testing.T) {
	manager := artifacts.NewDockerArtifactsManager(".artifacts")
	err := NewDockerRunner("Non existing artifact publish", manager, DockerRunnerOptions{ShowImagePull: false, Stdout: nil, Stderr: nil}).
		WithImage("docker.io/alpine").
		CreatesArtifacts([]string{"testing123"}).
		Run(context.Background())
	assert.ErrorContains(t, err, "unable to publish artifacts")
	teardown(t)
}

func TestTimeout(t *testing.T) {
	manager := artifacts.NewDockerArtifactsManager(".artifacts")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := NewDockerRunner("Test Timeout", manager, DockerRunnerOptions{ShowImagePull: false, Stdout: nil, Stderr: nil}).
		WithImage("docker.io/alpine").
		WithCmd([]string{"sleep", "60"}).
		Run(ctx)
	assert.ErrorContains(t, err, "container test-timeout")
	teardown(t)
}

func testImageOutput(t *testing.T, b *bytes.Buffer) bool {
	str := b.String()
	lines := strings.Split(str, "\n")

	if len(lines) < 1 {
		t.Error("output lines less than expected")
		return false
	}
	name := strings.Split(lines[0], "=")
	if len(name) != 2 {
		t.Error("name field not found")
		return false
	}

	return (strings.Compare(strings.Trim(name[1], "\""), "Alpine Linux") == 0)

}

func testVariableOutput(t *testing.T, b *bytes.Buffer) bool {
	str := b.String()
	str = regexp.MustCompile(`[^a-zA-Z0-9 ]+`).ReplaceAllString(str, "")
	return (strings.Compare(strings.TrimSpace(str), "TESTING") == 0)
}

func testArtifactCreation(t *testing.T, b *bytes.Buffer) bool {
	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return false
	}

	files, err := os.ReadDir(filepath.Join(wd, ".artifacts"))
	if err != nil {
		t.Error(err)
		return false
	}
	for _, f := range files {
		err := utils.DecompressTar(filepath.Join(wd, ".artifacts", f.Name()), filepath.Join(wd, ".artifacts"))
		if err != nil {
			t.Error(err)
		}

		logFile, err := os.ReadFile(filepath.Join(wd, ".artifacts", "log.txt"))
		if err != nil {
			t.Error(err)
		}
		testing := regexp.MustCompile(`[^a-zA-Z0-9 ]+`).ReplaceAllString(string(logFile), "")
		if strings.Compare(strings.TrimSpace(testing), "TESTING") == 0 {
			return true
		}
	}
	return false
}

// func testTimeoutOutput(t *testing.T, b *bytes.Buffer) bool {
// 	str := b.String()
// 	str = regexp.MustCompile(`[^a-zA-Z0-9 ]+`).ReplaceAllString(str, "")
// 	return (strings.Compare(strings.TrimSpace(str), "context timed out") == 0)
// }
