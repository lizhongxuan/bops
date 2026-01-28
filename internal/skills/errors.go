package skills

import (
	"fmt"
	"strings"
)

// LoadError captures a structured skill loading error.
type LoadError struct {
	Skill   string
	Path    string
	Field   string
	Message string
	Hint    string
	Err     error
}

func (e *LoadError) Error() string {
	parts := []string{}
	if e.Skill != "" {
		parts = append(parts, fmt.Sprintf("skill=%s", e.Skill))
	}
	if e.Path != "" {
		parts = append(parts, fmt.Sprintf("path=%s", e.Path))
	}
	if e.Field != "" {
		parts = append(parts, fmt.Sprintf("field=%s", e.Field))
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Hint != "" {
		parts = append(parts, fmt.Sprintf("hint=%s", e.Hint))
	}
	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("err=%s", e.Err.Error()))
	}
	return strings.Join(parts, " ")
}

func (e *LoadError) Unwrap() error {
	return e.Err
}

func NewLoadError(skill, path, field, message, hint string, err error) *LoadError {
	return &LoadError{
		Skill:   skill,
		Path:    path,
		Field:   field,
		Message: message,
		Hint:    hint,
		Err:     err,
	}
}
