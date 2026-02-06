package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bops/runner/logging"
	"go.uber.org/zap"
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
			logging.L().Debug("state load missing", zap.String("path", s.Path))
			return StateFile{}, nil
		}
		logging.L().Debug("state load failed", zap.String("path", s.Path), zap.Error(err))
		return StateFile{}, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	var data StateFile
	if err := dec.Decode(&data); err != nil {
		logging.L().Debug("state decode failed", zap.String("path", s.Path), zap.Error(err))
		return StateFile{}, err
	}
	logging.L().Debug("state loaded", zap.String("path", s.Path), zap.Int("runs", len(data.Runs)))
	return data, nil
}

func (s *FileStore) Save(state StateFile) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logging.L().Debug("state mkdir failed", zap.String("dir", dir), zap.Error(err))
		return err
	}

	state.UpdatedAt = time.Now().UTC()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		logging.L().Debug("state encode failed", zap.String("path", s.Path), zap.Error(err))
		return err
	}

	tmp, err := os.CreateTemp(dir, "state-*.json")
	if err != nil {
		logging.L().Debug("state tempfile failed", zap.String("dir", dir), zap.Error(err))
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		logging.L().Debug("state write failed", zap.String("tmp", tmpPath), zap.Error(err))
		return err
	}
	if err := tmp.Close(); err != nil {
		logging.L().Debug("state close failed", zap.String("tmp", tmpPath), zap.Error(err))
		return err
	}

	if err := os.Rename(tmpPath, s.Path); err != nil {
		logging.L().Debug("state rename failed", zap.String("tmp", tmpPath), zap.String("path", s.Path), zap.Error(err))
		return fmt.Errorf("persist state: %w", err)
	}

	logging.L().Debug("state saved", zap.String("path", s.Path), zap.Int("runs", len(state.Runs)))
	return nil
}
