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
	"github.com/vito/progrock/ui"
)

func electRecorder() (ui.Reader, *progrock.Recorder, error) {
	socketPath, err := xdg.StateFile(fmt.Sprintf("bass/recorder.%d.sock", syscall.Getpgrp()))
	if err != nil {
		return nil, nil, err
	}

	var r ui.Reader
	var w progrock.Writer
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		r = nil // don't display any progress; send it to the leader
		w, err = progrock.DialRPC("unix", socketPath)
	} else {
		r, w, err = progrock.ServeRPC(l)
	}

	return r, progrock.NewRecorder(w), err
}

func cleanupRecorder() error {
	socketPath, err := xdg.StateFile(fmt.Sprintf("bass/recorder.%d.sock", syscall.Getpgrp()))
	if err != nil {
		return err
	}

	return os.RemoveAll(socketPath)
}
