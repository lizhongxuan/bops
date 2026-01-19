package marketplace

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Installer struct {
	Root string
}

func NewInstaller(root string) *Installer {
	return &Installer{Root: root}
}

func (i *Installer) InstallFromPath(path string) (string, error) {
	if i.Root == "" {
		return "", fmt.Errorf("marketplace root is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	name := info.Name()
	dest := filepath.Join(i.Root, name)
	if err := copyDir(path, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		info, err := d.Info()
		if err != nil {
			return err
		}
		dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}
		return nil
	})
}
