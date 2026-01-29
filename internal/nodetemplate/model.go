package nodetemplate

type NodeSpec struct {
	Type    string         `json:"type" yaml:"type"`
	Name    string         `json:"name" yaml:"name"`
	Action  string         `json:"action" yaml:"action"`
	With    map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
	Targets []string       `json:"targets,omitempty" yaml:"targets,omitempty"`
}

type Template struct {
	Name        string   `json:"name" yaml:"name"`
	Category    string   `json:"category" yaml:"category"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Node        NodeSpec `json:"node" yaml:"node"`
}

type Summary struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Action      string   `json:"action,omitempty"`
	Node        NodeSpec `json:"node,omitempty"`
}
