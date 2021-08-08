package docker

import (
	"path/filepath"

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
	return filepath.Abs(
		filepath.Join(
			append(
				[]string{config.Data},
				path...,
			)...,
		),
	)
}
