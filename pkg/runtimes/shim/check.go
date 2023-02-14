package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

func check(args []string) error {
	logger := StdLogger(logLevel)

	if len(args) == 0 {
		return fmt.Errorf("usage: check <host> name:port [name:port ...]")
	}

	host, ports := args[0], args[1:]

	for _, nameAndPort := range ports {
		name, port, ok := strings.Cut(nameAndPort, ":")
		if !ok {
			return fmt.Errorf("port must be in form name:number: %s", nameAndPort)
		}

		logger := logger.With(zap.String("name", name), zap.String("port", port))

		logger.Debug("polling for port")

		pollAddr := net.JoinHostPort(host, port)

		reached, err := pollForPort(logger, pollAddr)
		if err != nil {
			return fmt.Errorf("poll %s: %w", name, err)
		}

		logger.Info("port is up", zap.String("reached", reached))
	}

	return nil
}

func pollForPort(logger *zap.Logger, addr string) (string, error) {
	retry := backoff.NewExponentialBackOff()
	retry.InitialInterval = 100 * time.Millisecond

	dialer := net.Dialer{
		Timeout: time.Second,
	}

	var reached string
	err := backoff.Retry(func() error {
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			logger.Debug("failed to dial", zap.Duration("elapsed", retry.GetElapsedTime()), zap.Error(err))
			return err
		}

		host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			// don't know how this would happen but it's likely not recoverable
			logger.Error("malformed host:port", zap.Error(err))
			return backoff.Permanent(err)
		}

		reached = host

		_ = conn.Close()

		return nil
	}, retry)
	if err != nil {
		return "", err
	}

	return reached, nil
}
