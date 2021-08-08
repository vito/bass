package docker

import (
	"fmt"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/vito/bass"
)

type RuntimeConfig struct {
	Data string `json:"data,omitempty"`
}

const artifactsDir = "artifacts"
const locksDir = "locks"
const responsesDir = "responses"
const logsDir = "logs"

func (config RuntimeConfig) ArtifactsPath(id string, path bass.FilesystemPath) (string, error) {
	return config.path(artifactsDir, id, path.FromSlash())
}

func (config RuntimeConfig) LockPath(id string) (string, error) {
	return config.path(locksDir, id+".lock")
}

func (config RuntimeConfig) ResponsePath(id string) (string, error) {
	return config.path(responsesDir, id)
}

func (config RuntimeConfig) LogPath(id string) (string, error) {
	return config.path(logsDir, id)
}

func (config RuntimeConfig) path(path ...string) (string, error) {
	dataRoot, err := homedir.Expand(config.Data)
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	expanded := filepath.Join(append([]string{dataRoot}, path...)...)

	return filepath.Abs(expanded)
}
