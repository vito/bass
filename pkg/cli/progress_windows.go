package cli

import (
	"github.com/vito/progrock"
)

func electRecorder() (*progrock.Tape, *progrock.Recorder, error) {
	tape := progrock.NewTape()
	return tape, progrock.NewRecorder(tape), nil
}

func cleanupRecorder() error {
	return nil
}
