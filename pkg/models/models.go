package models

type Stage string
type Variable map[string]string

type JobFile struct {
	Stages []Stage `yaml:"stages" validate:"required,dive"`
	Jobs   []Job   `yaml:"jobs" validate:"required,dive"`
}

type Job struct {
	Name      string     `yaml:"name" validate:"required,alphanum"`
	Src       string     `yaml:"src"`
	Stage     Stage      `yaml:"stage" validate:"required"`
	Variables []Variable `yaml:"variables"`
	Image     string     `yaml:"image" validate:"required"`
	Script    []string   `yaml:"script"`
	Artifacts []string   `yaml:"artifacts"`
}
