package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type StateFile struct {
	UpdatedAt time.Time  `json:"updated_at"`
	Runs      []RunState `json:"runs"`
}

type Store interface {
	Load() (StateFile, error)
	Save(state StateFile) error
}

type FileStore struct {
	Path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{Path: path}
}

func (s *FileStore) Load() (StateFile, error) {
	file, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return StateFile{}, nil
		}
		return StateFile{}, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	var data StateFile
	if err := dec.Decode(&data); err != nil {
		return StateFile{}, err
	}
	return data, nil
}

func (s *FileStore) Save(state StateFile) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	state.UpdatedAt = time.Now().UTC()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "state-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.Path); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}

	return nil
}
