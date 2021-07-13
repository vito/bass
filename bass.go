package bass

import (
	"errors"
	"io"
	"os"
)

func NewStandardEnv() *Env {
	return NewEnv(ground)
}

func EvalFile(env *Env, filePath string) (Value, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return EvalReader(env, file)
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

func Trampoline(val Value) (Value, error) {
	for {
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
}
