package runtimes

import (
	"context"
	"io"

	"github.com/vito/bass"
)

type Runtime interface {
	Run(context.Context, bass.Workload) error
	Response(context.Context, io.Writer, bass.Workload) error
	Env(context.Context, bass.Workload) (*bass.Env, error)
	Export(context.Context, io.Writer, bass.Workload, bass.FilesystemPath) error
}
