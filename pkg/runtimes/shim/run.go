package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/sys/reaper"
	"github.com/moby/sys/mountinfo"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type Command struct {
	Args  []string `json:"args"`
	Stdin []byte   `json:"stdin"`
	Env   []string `json:"env"`
	Dir   *string  `json:"dir"`
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: run <cmd.json>")
	}

	logger := StdLogger(logLevel)

	if debug {
		hn, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("get hostname: %w", err)
		}

		logger.Debug("starting", zap.String("hostname", hn))

		logIPs(logger)
	}

	if err := installCert(); err != nil {
		return fmt.Errorf("install bass CA: %w", err)
	}

	if err := spawnReaper(); err != nil {
		return fmt.Errorf("reap: %w", err)
	}

	cmdPath := args[0]

	cmdPayload, err := os.ReadFile(cmdPath)
	if err != nil {
		return fmt.Errorf("read cmd: %w", err)
	}

	var cmd Command
	err = json.Unmarshal(cmdPayload, &cmd)
	if err != nil {
		return fmt.Errorf("unmarshal cmd: %w", err)
	}

	err = os.Remove(cmdPath)
	if err != nil {
		return fmt.Errorf("burn after reading: %w", err)
	}

	stdoutPath := os.Getenv("_BASS_OUTPUT")
	os.Unsetenv("_BASS_OUTPUT")

	var stdout io.Writer = os.Stdout
	if stdoutPath != "" {
		response, err := os.Create(stdoutPath)
		if err != nil {
			return fmt.Errorf("create output error: %w", err)
		}

		defer response.Close()

		stdout = io.MultiWriter(stdout, response)
	}

	for _, e := range cmd.Env {
		segs := strings.SplitN(e, "=", 2)
		if len(segs) != 2 {
			return fmt.Errorf("malformed env: %s", e)
		}

		os.Setenv(segs[0], segs[1])
	}

	bin := cmd.Args[0]
	argv := cmd.Args[1:]
	execCmd := exec.Command(bin, argv...)
	if cmd.Dir != nil {
		execCmd.Dir = *cmd.Dir
	}
	execCmd.Stdin = bytes.NewBuffer(cmd.Stdin)
	execCmd.Stdout = stdout
	execCmd.Stderr = os.Stderr

	ch, err := reaper.Default.Start(execCmd)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}

	status, err := reaper.Default.Wait(execCmd, ch)
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if status != 0 {
		// propagate exit status
		os.Exit(status)
		return nil
	}

	err = normalizeTimes(".")
	if err != nil {
		return fmt.Errorf("failed to normalize timestamps: %w", err)
	}

	return nil
}

func spawnReaper() error {
	logger := StdLogger(logLevel)

	reaper.SetSubreaper(1)

	children := make(chan os.Signal, 32)
	signal.Notify(children, syscall.SIGCHLD)

	go func() {
		for range children {
			err := reaper.Reap()
			if err != nil {
				logger.Warn("failed to reap", zap.Error(err))
			}
		}
	}()

	return nil
}

var epoch = time.Date(1985, 10, 26, 8, 15, 0, 0, time.UTC)

func normalizeTimes(root string) error {
	logger := StdLogger(logLevel)

	skipped := 0
	unchanged := 0
	changed := 0
	start := time.Now()
	tspec := unix.NsecToTimespec(epoch.UnixNano())
	targetTime := []unix.Timespec{tspec, tspec}
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path != root && info.IsDir() {
			mp, err := mountinfo.Mounted(path)
			if err != nil {
				return fmt.Errorf("check mounted: %w", err)
			}

			if mp {
				logger.Debug("skipping mountpoint", zap.String("path", path))
				skipped++
				return fs.SkipDir
			}
		}

		if info.ModTime().Equal(epoch) {
			unchanged++
			return nil
		}

		changed++

		logger.Debug("chtimes",
			zap.String("path", path),
			zap.Time("from", info.ModTime()),
			zap.Time("to", epoch))

		err = unix.UtimesNanoAt(unix.AT_FDCWD, path, targetTime, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			return fmt.Errorf("chtimes: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	logger.Info("times normalized",
		zap.Duration("took", time.Since(start)),
		zap.Int("changed", changed),
		zap.Int("unchanged", unchanged),
		zap.Int("skipped", skipped),
	)

	return nil
}
