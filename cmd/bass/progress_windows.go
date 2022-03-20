package main

import (
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

func electRecorder() (ui.Reader, *progrock.Recorder, error) {
	r, w := progrock.Pipe()
	return r, progrock.NewRecorder(w), nil
}

func cleanupRecorder() error {
	return nil
}
