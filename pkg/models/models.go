package models

type Stage string

type JobFile struct {
	Stages []Stage `yaml:"stages"`
	Jobs   []Job   `yaml:"jobs"`
}

type Job struct {
	Name      string   `yaml:"name"`
	Stage     Stage    `yaml:"stage"`
	Variables []string `yaml:"variables"`
	Image     string   `yaml:"image"`
	Script    []string `yaml:"script"`
	Artifacts []string `yaml:"artifacts"`
}
