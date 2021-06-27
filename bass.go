package bass

import (
	"errors"
	"io"
)

func New() *Env {
	return NewEnv(ground)
}

func EvalReader(e *Env, r io.Reader) (Value, error) {
	reader := NewReader(r)

	var res Value
	for {
		val, err := reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		res, err = val.Eval(e)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}
