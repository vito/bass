package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/progrock"
)

func bump(ctx context.Context) error {
	return withProgress(ctx, "export", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		lockContent, err := os.ReadFile(bumpLock)
		if err != nil {
			return err
		}

		var lf bass.LockfileContent
		err = bass.UnmarshalJSON(lockContent, &lf)
		if err != nil {
			return err
		}

		for thunkFn, pairs := range lf.Data {
			segs := strings.SplitN(thunkFn, ":", 2)
			if len(segs) != 2 {
				return fmt.Errorf("malformed bass.lock key: %q", thunkFn)
			}

			thunkID := segs[0]
			fn := bass.Symbol(segs[1])
			thunk := lf.Thunks[thunkID]

			scope, err := bass.Bass.Load(ctx, thunk)
			if err != nil {
				return err
			}

			var comb bass.Combiner
			err = scope.GetDecode(fn, &comb)
			if err != nil {
				return err
			}

			for i, pair := range pairs {
				res, err := bass.Trampoline(ctx, comb.Call(ctx, pair.Input.Value, bass.NewEmptyScope(), bass.Identity))
				if err != nil {
					return err
				}

				// update reference inline
				pairs[i].Output.Value = res
			}
		}

		lockFile, err := os.Create(bumpLock)
		if err != nil {
			return err
		}

		defer lockFile.Close()

		enc := bass.NewEncoder(lockFile)
		enc.SetIndent("", "  ")
		err = enc.Encode(lf)
		if err != nil {
			return err
		}

		return nil
	})
}
