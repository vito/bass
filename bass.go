package bass

import (
	"context"
	"errors"
	"io"
	"os"
)

func NewStandardEnv() *Env {
	return NewEnv(ground)
}

func EvalFile(ctx context.Context, env *Env, filePath string, args ...Value) (Value, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(ctx, env, file)
}

func EvalReader(ctx context.Context, e *Env, r io.Reader, name ...string) (Value, error) {
	reader := NewReader(r, name...)

	var res Value
	for {
		val, err := reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		rdy := val.Eval(ctx, e, Identity)

		res, err = Trampoline(ctx, rdy)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func Trampoline(ctx context.Context, val Value) (Value, error) {
	for ctx.Err() == nil {
		var cont ReadyCont
		if err := val.Decode(&cont); err != nil {
			return val, nil
		}

		var err error
		val, err = cont.Go()
		if err != nil {
			return nil, err
		}
	}

	return nil, ErrInterrupted
}
