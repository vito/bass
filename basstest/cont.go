package basstest

import (
	"context"

	"github.com/vito/bass"
)

func Eval(env *bass.Env, val bass.Value) (bass.Value, error) {
	return EvalContext(context.Background(), env, val)
}

func EvalContext(ctx context.Context, env *bass.Env, val bass.Value) (bass.Value, error) {
	return bass.Trampoline(ctx, val.Eval(ctx, env, bass.Identity))
}

func Call(comb bass.Combiner, env *bass.Env, val bass.Value) (bass.Value, error) {
	ctx := context.Background()
	return bass.Trampoline(ctx, comb.Call(ctx, val, env, bass.Identity))
}
