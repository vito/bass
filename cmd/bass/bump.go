package main

import (
	"context"
	"os"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/proto"
	"github.com/vito/progrock"
	"google.golang.org/protobuf/encoding/prototext"
)

func bump(ctx context.Context) error {
	return withProgress(ctx, "export", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		lockContent, err := os.ReadFile(bumpLock)
		if err != nil {
			return err
		}

		content := &proto.Memosphere{}
		err = prototext.Unmarshal(lockContent, content)
		if err != nil {
			return err
		}

		for _, memo := range content.Memos {
			thunk := bass.Thunk{}
			err := thunk.UnmarshalProto(memo.Module)
			if err != nil {
				return err
			}

			scope, err := bass.Bass.Load(ctx, thunk)
			if err != nil {
				return err
			}

			for _, call := range memo.Calls {
				binding := bass.Symbol(call.Binding)

				var comb bass.Combiner
				err = scope.GetDecode(binding, &comb)
				if err != nil {
					return err
				}

				for _, res := range call.Results {
					input, err := bass.FromProto(res.Input)
					if err != nil {
						return err
					}

					out, err := bass.Trampoline(ctx, comb.Call(ctx, input, bass.NewEmptyScope(), bass.Identity))
					if err != nil {
						return err
					}

					output, err := bass.MarshalProto(out)
					if err != nil {
						return err
					}

					res.Output = output
				}
			}
		}

		payload, err := prototext.MarshalOptions{Indent: "  "}.Marshal(content)
		if err != nil {
			return err
		}

		return os.WriteFile(bumpLock, payload, 0644)
	})
}
