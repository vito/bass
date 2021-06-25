package main

import (
	"fmt"
	"os"

	"github.com/vito/bass"
)

func main() {
	reader := bass.NewReader(os.Stdin)

	env := bass.New()

	for {
		form, err := reader.Next()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		res, err := form.Eval(env)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		fmt.Println(res)
	}
}
