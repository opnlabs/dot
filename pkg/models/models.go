package models

type Stage string
type Variable map[string]string

type JobFile struct {
	Stages []Stage `yaml:"stages"`
	Jobs   []Job   `yaml:"jobs"`
}

type Job struct {
	Name      string     `yaml:"name"`
	Src       string     `yaml:"src"`
	Stage     Stage      `yaml:"stage"`
	Variables []Variable `yaml:"variables"`
	Image     string     `yaml:"image"`
	Script    []string   `yaml:"script"`
	Artifacts []string   `yaml:"artifacts"`
}
