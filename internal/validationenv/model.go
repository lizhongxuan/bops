package validationenv

type EnvType string

const (
	EnvTypeContainer EnvType = "container"
	EnvTypeSSH       EnvType = "ssh"
	EnvTypeAgent     EnvType = "agent"
)

type ValidationEnv struct {
	Name        string            `json:"name" yaml:"name"`
	Type        EnvType           `json:"type" yaml:"type"`
	Description string            `json:"description" yaml:"description"`
	Labels      map[string]string `json:"labels" yaml:"labels"`

	Image string `json:"image" yaml:"image"`

	Host   string `json:"host" yaml:"host"`
	User   string `json:"user" yaml:"user"`
	SSHKey string `json:"ssh_key" yaml:"ssh_key"`

	AgentAddress string `json:"agent_address" yaml:"agent_address"`
}
