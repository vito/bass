package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/proto"
	"github.com/vito/progrock"
	"google.golang.org/protobuf/encoding/protojson"
)

func bump(ctx context.Context) error {
	return withProgress(ctx, "export", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		lockContent, err := os.ReadFile(bumpLock)
		if err != nil {
			return err
		}

		ms := proto.NewMemosphere()
		err = protojson.Unmarshal(lockContent, ms)
		if err != nil {
			return err
		}

		for thunkFn, pairs := range ms.Data {
			segs := strings.SplitN(thunkFn, ":", 2)
			if len(segs) != 2 {
				return fmt.Errorf("malformed bass.lock key: %q", thunkFn)
			}

			thunkID := segs[0]
			fn := bass.Symbol(segs[1])

			var thunk bass.Thunk
			if err := thunk.UnmarshalProto(ms.Modules[thunkID]); err != nil {
				return err
			}

			scope, err := bass.Bass.Load(ctx, thunk)
			if err != nil {
				return err
			}

			var comb bass.Combiner
			err = scope.GetDecode(fn, &comb)
			if err != nil {
				return err
			}

			for i, pair := range pairs.GetMemos() {
				input, err := bass.FromProto(pair.Input)
				if err != nil {
					return err
				}

				res, err := bass.Trampoline(ctx, comb.Call(ctx, input, bass.NewEmptyScope(), bass.Identity))
				if err != nil {
					return err
				}

				output, err := bass.MarshalProto(res)
				if err != nil {
					return err
				}

				// update reference inline
				pairs.Memos[i].Output = output
			}
		}

		content, err := protojson.MarshalOptions{Indent: "  "}.Marshal(ms)
		if err != nil {
			return err
		}

		return os.WriteFile(bumpLock, content, 0644)
	})
}
