package skills

import "path/filepath"

const DefaultRoot = "./skills"

type ConflictPolicy string

const (
	ConflictError        ConflictPolicy = "error"
	ConflictOverwrite    ConflictPolicy = "overwrite"
	ConflictKeepExisting ConflictPolicy = "keep"
)

// ResolveRoot resolves the skills root directory from a base directory and config value.
func ResolveRoot(baseDir, configured string) string {
	root := configured
	if root == "" {
		root = DefaultRoot
	}
	if filepath.IsAbs(root) {
		return filepath.Clean(root)
	}
	if baseDir == "" {
		return filepath.Clean(root)
	}
	return filepath.Clean(filepath.Join(baseDir, root))
}

// ResolveSkillDir returns the on-disk directory for a skill name under the root.
func ResolveSkillDir(root, skillName string) string {
	return filepath.Clean(filepath.Join(root, skillName))
}
