package dot

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/go-playground/validator/v10"
	"github.com/opnlabs/dot/pkg/artifacts"
	"github.com/opnlabs/dot/pkg/models"
	"github.com/opnlabs/dot/pkg/runner"
	"github.com/opnlabs/dot/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var (
	jobFilePath          string
	mountDockerSocket    bool
	envVars              []string
	environmentVariables []models.Variable = make([]models.Variable, 0)
	username             string
	password             string
	validate             *validator.Validate = validator.New(validator.WithRequiredStructEnabled())
)

var rootCmd = &cobra.Command{
	Use:   "dot",
	Short: "Dot is a minimal CI",
	Long: `Dot is a minimal CI that runs jobs defined in a file ( default dot.yml )
inside docker containers. Jobs can be divided into stages where jobs within a stage are executed
concurrently.`,

	Run: func(cmd *cobra.Command, args []string) {

		if len(envVars) > 0 {
			for _, v := range envVars {
				variables := strings.Split(v, "=")
				if len(variables) != 2 {
					log.Fatalf("variables should be defined as KEY=VALUE: %s", v)
				}

				m := make(map[string]any)
				m[variables[0]] = variables[1]
				environmentVariables = append(environmentVariables, m)
			}
		}

		run()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&jobFilePath, "job-file-path", "f", "dot.yml", "Path to the job file.")
	rootCmd.Flags().BoolVarP(&mountDockerSocket, "mount-docker-socket", "m", false, "Mount docker socket. Required to run containers from dot.")
	rootCmd.Flags().StringVarP(&username, "registry-username", "u", "", "Username for the container registry")
	rootCmd.Flags().StringVarP(&password, "registry-password", "p", "", "Password / Token for the container registry")

	rootCmd.Flags().StringArrayVarP(&envVars, "environment-variable", "e", make([]string, 0), "Environment variables. KEY=VALUE")

	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command for dot.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run() {
	ctx := context.Background()
	contents, err := os.ReadFile(filepath.Clean(jobFilePath))
	if err != nil {
		log.Fatal(err)
	}

	var jobFile models.JobFile
	err = yaml.Unmarshal(contents, &jobFile)
	if err != nil {
		log.Fatal(err)
	}

	err = validate.Struct(jobFile)
	if err != nil {
		log.Fatalf("Err(s):\n%+v\n", err)
	}

	stageMap := make(map[models.Stage][]models.Job)
	for _, v := range jobFile.Stages {
		stageMap[v] = make([]models.Job, 0)
	}

	for _, v := range jobFile.Jobs {
		if _, ok := stageMap[v.Stage]; !ok {
			log.Fatalf("stage not defined: %s", v.Stage)
		}

		// Create expr program with the variables passed as env
		if len(v.Condition) == 0 {
			v.Condition = `true`
		}

		env := make(map[string]any)
		for _, entries := range v.Variables {
			if len(entries) > 1 {
				log.Fatal("variables should be defined as a key value pair")
			}
			for k, value := range entries {
				env[k] = value
			}
		}

		p, err := expr.Compile(v.Condition, expr.Env(env), expr.AsBool())
		if err != nil {
			log.Fatalf("condition evaluation failed for job %s: %v", v.Name, err)
		}
		output, err := expr.Run(p, env)
		if err != nil {
			log.Fatalf("condition evaluation failed for job %s: %v", v.Name, err)
		}

		// Only append to stageMap if the condition evaluates to true
		if output.(bool) {
			stageMap[v.Stage] = append(stageMap[v.Stage], v)
		}

	}

	dockerArtifactManager := artifacts.NewDockerArtifactsManager(".artifacts")

	for _, v := range jobFile.Stages {
		var eg errgroup.Group
		for _, job := range stageMap[v] {
			jobCtx, cancel := context.WithTimeout(ctx, time.Hour)
			defer cancel()

			func(job models.Job) {
				eg.Go(func() error {
					return runner.NewDockerRunner(job.Name, dockerArtifactManager,
						runner.DockerRunnerOptions{
							ShowImagePull:     true,
							Stdout:            utils.NewColorLogger(job.Name, os.Stdout, true),
							Stderr:            utils.NewColorLogger(job.Name, os.Stderr, false),
							MountDockerSocket: mountDockerSocket}).
						WithImage(job.Image).
						WithSrc(job.Src).
						WithCmd(job.Script).
						WithEntrypoint(job.Entrypoint).
						WithEnv(append(job.Variables, environmentVariables...)).
						WithCredentials(username, password).
						CreatesArtifacts(job.Artifacts).Run(jobCtx)
				})
			}(job)
		}
		err := eg.Wait()
		if err != nil {
			log.Fatal(err)
		}
	}
}
