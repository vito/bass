package basstest

import (
	"context"

	"github.com/vito/bass/bass"
)

func Eval(scope *bass.Scope, val bass.Value) (bass.Value, error) {
	return EvalContext(context.Background(), scope, val)
}

func EvalContext(ctx context.Context, scope *bass.Scope, val bass.Value) (bass.Value, error) {
	return bass.Trampoline(ctx, val.Eval(ctx, scope, bass.Identity))
}

func Call(comb bass.Combiner, scope *bass.Scope, val bass.Value) (bass.Value, error) {
	ctx := context.Background()
	return bass.Trampoline(ctx, comb.Call(ctx, val, scope, bass.Identity))
}
