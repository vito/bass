//go:build !windows
// +build !windows

package cli

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/vito/progrock"
)

func electRecorder() (*progrock.Tape, *progrock.Recorder, error) {
	socketPath, err := xdg.StateFile(fmt.Sprintf("bass/recorder.%d.sock", syscall.Getpgrp()))
	if err != nil {
		return nil, nil, err
	}

	var Tape *progrock.Tape
	var w progrock.Writer
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		w, err = progrock.DialRPC("unix", socketPath)
	} else {
		Tape = progrock.NewTape()
		w, err = progrock.ServeRPC(l, Tape)
	}

	return Tape, progrock.NewRecorder(w), err
}

func cleanupRecorder() error {
	socketPath, err := xdg.StateFile(fmt.Sprintf("bass/recorder.%d.sock", syscall.Getpgrp()))
	if err != nil {
		return err
	}

	return os.RemoveAll(socketPath)
}
