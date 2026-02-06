package scriptstore

type Script struct {
	Name        string   `json:"name" yaml:"name"`
	Language    string   `json:"language" yaml:"language"`
	Description string   `json:"description" yaml:"description"`
	Tags        []string `json:"tags" yaml:"tags"`
	Content     string   `json:"content" yaml:"content"`
}
