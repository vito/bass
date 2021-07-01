package main

import (
	"os"

	"github.com/vito/bass"
)

func run(env *bass.Env, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = bass.EvalReader(env, file)
	if err != nil {
		return err
	}

	return nil
}
