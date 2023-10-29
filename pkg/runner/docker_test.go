package runner

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cvhariharan/done/pkg/artifacts"
	"github.com/cvhariharan/done/pkg/models"
	"github.com/cvhariharan/done/pkg/utils"
)

type Test struct {
	Name        string
	Manager     artifacts.ArtifactManager
	Image       string
	Src         string
	Script      []string
	Variables   []models.Variable
	Artifacts   []string
	Output      io.Writer
	Expectation func(*testing.T, *bytes.Buffer) bool
}

func teardown(tb testing.TB) {

	wd, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return
	}
	os.RemoveAll(filepath.Join(wd, ".done"))
	os.RemoveAll(filepath.Join(wd, ".artifacts"))
}

func TestRun(t *testing.T) {

	var b bytes.Buffer
	manager := artifacts.NewDockerArtifactsManager()

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
		},
	}

	for _, test := range tests {
		b.Truncate(0)
		NewDockerRunner(test.Name, test.Manager).
			WithImage(test.Image).
			WithSrc(test.Src).
			WithCmd(test.Script).
			WithEnv(test.Variables).
			CreatesArtifacts(test.Artifacts).Run(LogOptions{ShowImagePull: false, Stdout: test.Output, Stderr: os.Stderr})

		if !test.Expectation(t, &b) {
			t.Errorf("Test - %s: failed", test.Name)
		}
	}

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
