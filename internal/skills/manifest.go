package skills

type Manifest struct {
	Name        string       `json:"name" yaml:"name"`
	Version     string       `json:"version" yaml:"version"`
	Description string       `json:"description" yaml:"description"`
	Permissions []string     `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Profile     Profile      `json:"profile" yaml:"profile"`
	Memory      *Memory      `json:"memory,omitempty" yaml:"memory,omitempty"`
	Executables []Executable `json:"executables" yaml:"executables"`
}

type Profile struct {
	Role        string `json:"role" yaml:"role"`
	Instruction string `json:"instruction" yaml:"instruction"`
}

type Memory struct {
	Strategy string   `json:"strategy" yaml:"strategy"`
	Files    []string `json:"files" yaml:"files"`
}

type Executable struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string   `json:"type" yaml:"type"`
	Runner      string   `json:"runner,omitempty" yaml:"runner,omitempty"`
	Path        string   `json:"path,omitempty" yaml:"path,omitempty"`
	Command     string   `json:"command,omitempty" yaml:"command,omitempty"`
	Args        []string `json:"args,omitempty" yaml:"args,omitempty"`
	Parameters  any      `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}
