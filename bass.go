package bass

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
)

func EvalFile(ctx context.Context, scope *Scope, filePath string) (Value, error) {
	file, err := os.Open(path.Clean(filePath))
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(ctx, scope, file, filePath)
}

func EvalFSFile(ctx context.Context, scope *Scope, fs fs.FS, filePath string) (Value, error) {
	file, err := fs.Open(path.Clean(filePath))
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(ctx, scope, file, filePath)
}

func EvalString(ctx context.Context, e *Scope, str string, name ...string) (Value, error) {
	return EvalReader(ctx, e, bytes.NewBufferString(str), name...)
}

func EvalReader(ctx context.Context, e *Scope, r io.Reader, name ...string) (Value, error) {
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
