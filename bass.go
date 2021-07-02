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

		rdy := val.Eval(e, Identity)

		res, err = Trampoline(rdy)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func Trampoline(cont ReadyCont) (Value, error) {
	for {
		var val Value
		var err error
		val, cont, err = cont.Go()
		if err != nil {
			return nil, err
		}

		if val != nil {
			return val, nil
		}
	}
}
