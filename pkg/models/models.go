package models

type Stage string

// Variable represents a job variable as a key-value pair.
// Variables are defined as an array of key-value pairs, so each Variable map only has 1 entry.
type Variable map[string]any

// JobFile represents the dot.yml file
type JobFile struct {
	Stages []Stage `yaml:"stages" validate:"required,dive"`
	Jobs   []Job   `yaml:"jobs" validate:"required,dive"`
}

// Job represents a single job in a stage
type Job struct {
	Name       string     `yaml:"name" validate:"required"`
	Src        string     `yaml:"src"`
	Stage      Stage      `yaml:"stage" validate:"required"`
	Variables  []Variable `yaml:"variables"`
	Image      string     `yaml:"image" validate:"required"`
	Script     []string   `yaml:"script"`
	Entrypoint []string   `yaml:"entrypoint"`
	Artifacts  []string   `yaml:"artifacts"`
	Condition  string     `yaml:"condition"`
}
