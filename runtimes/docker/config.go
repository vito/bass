package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/vito/bass"
)

type RuntimeConfig struct {
	Data string `json:"data,omitempty"`
}

func (config RuntimeConfig) ArtifactsPath(id string, path bass.FilesystemPath) (string, error) {
	return config.path("artifacts", id, path.FromSlash())
}

func (config RuntimeConfig) LockPath(id string) (string, error) {
	return config.path("locks", id+".lock")
}

func (config RuntimeConfig) ResponsePath(id string) (string, error) {
	return config.path("responses", id+".json")
}

func (config RuntimeConfig) LogPath(id string) (string, error) {
	return config.path("logs", id)
}

func (config RuntimeConfig) path(path ...string) (string, error) {
	dataRoot, err := homedir.Expand(config.Data)
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	expanded := filepath.Join(append([]string{dataRoot}, path...)...)

	err = os.MkdirAll(filepath.Dir(expanded), 0700)
	if err != nil {
		return "", fmt.Errorf("create parent dir: %w", err)
	}

	return filepath.Abs(expanded)
}
