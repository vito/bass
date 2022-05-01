package bass

import (
	"context"
	"errors"
)

func RunMain(ctx context.Context, scope *Scope, args ...Value) error {
	var comb Combiner
	if err := scope.GetDecode(RunBindingMain, &comb); err != nil {
		var unb UnboundError
		if errors.As(err, &unb) {
			return nil
		}

		return err
	}

	_, err := Trampoline(ctx, comb.Call(ctx, NewList(args...), scope, Identity))
	return err
}
