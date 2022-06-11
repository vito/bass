package bass

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path"
)

func EvalFile(ctx context.Context, scope *Scope, filePath string, source Readable) (Value, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(ctx, scope, file, source)
}

func EvalFSFile(ctx context.Context, scope *Scope, source *FSPath) (Value, error) {
	file, err := source.FS.Open(path.Clean(source.Path.Slash()))
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(ctx, scope, file, source)
}

func EvalString(ctx context.Context, e *Scope, str string, source Readable) (Value, error) {
	return EvalReader(ctx, e, bytes.NewBufferString(str), source)
}

func EvalReader(ctx context.Context, e *Scope, r io.Reader, source Readable) (Value, error) {
	reader := NewReader(r, source)

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
	var err error
	for ctx.Err() == nil {
		cont, ok := val.(ReadyCont)
		if !ok {
			return val, nil
		}

		val, err = cont.Go()
		if err != nil {
			return nil, err
		}
	}

	return nil, ErrInterrupted
}
