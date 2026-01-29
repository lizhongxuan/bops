package nodetemplate

type NodeSpec struct {
	Type string         `json:"type" yaml:"type"`
	Name string         `json:"name" yaml:"name"`
	Data map[string]any `json:"data,omitempty" yaml:"data,omitempty"`
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
	Node        NodeSpec `json:"node,omitempty"`
}
