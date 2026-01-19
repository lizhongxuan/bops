package ai

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Prompt  string         `json:"prompt"`
	Context map[string]any `json:"context,omitempty"`
}

type FixRequest struct {
	YAML   string   `json:"yaml"`
	Issues []string `json:"issues,omitempty"`
}
