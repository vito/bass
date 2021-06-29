package main

import (
	"fmt"
	"os"

	"github.com/vito/bass"
)

func run(env *bass.Env, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	val, err := bass.EvalReader(env, file)
	if err != nil {
		return err
	}

	fmt.Println(val)
	return nil
}
