package runtimes

import (
	"context"
	"io"

	"github.com/vito/bass"
)

type Runtime interface {
	Run(context.Context, string, bass.Workload) error
	Response(context.Context, io.Writer, string, bass.Workload) error
	Export(context.Context, io.Writer, string, bass.Workload, bass.FilesystemPath) error
}
