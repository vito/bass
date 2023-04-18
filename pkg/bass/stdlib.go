package bass

import (
	"context"
	"fmt"

	"github.com/mattn/go-colorable"
	"github.com/morikuni/aec"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/bass/std"
	"go.uber.org/zap"
)

func init() {
	for _, lib := range []string{
		"root.bass",
		"streams.bass",
		"run.bass",
		"paths.bass",
		"bool.bass",
	} {
		stderr := colorable.NewColorableStderr()
		ctx := context.Background()
		ctx = ioctx.StderrToContext(ctx, stderr)
		ctx = zapctx.ToContext(ctx, StdLogger(zap.DebugLevel))

		source := NewFSPath(std.FS, FileOrDirPath{File: &FilePath{Path: lib}})
		_, err := EvalFSFile(ctx, Ground, source)
		if err != nil {
			fmt.Fprintf(stderr, aec.YellowF.Apply("eval ground %s: %s\n"), lib, err)
		}
	}
}
