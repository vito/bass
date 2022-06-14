package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sync/errgroup"
)

var defaultKeys = []string{
	"id_dsa",
	"id_ecdsa",
	"id_ecdsa_sk",
	"id_ed25519",
	"id_ed25519_sk",
	"id_rsa",
}

func runnerLoop(ctx context.Context, sshAddr string, assoc []runtimes.Assoc) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return withProgress(ctx, "runner", func(ctx context.Context, bassVertex *progrock.VertexRecorder) (err error) {
		exp := backoff.NewExponentialBackOff()
		exp.MaxElapsedTime = 0 // https://www.youtube.com/watch?v=6BtuqUX934U
		return backoff.Retry(func() error {
			return runner(ctx, sshAddr, assoc)
		}, backoff.WithContext(exp, ctx))
	})
}

func runner(ctx context.Context, sshAddr string, assoc []runtimes.Assoc) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	logger := zapctx.FromContext(ctx)

	osuser, err := user.Current()
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	login, rest, ok := strings.Cut(sshAddr, "@")
	if ok {
		sshAddr = rest
	} else {
		login = osuser.Username
	}

	host, port, err := net.SplitHostPort(sshAddr)
	if err != nil {
		host = sshAddr
		port = "6455"
	}

	hostKeyCallback, err := knownhosts.New(filepath.Join(osuser.HomeDir, ".ssh", "known_hosts"))
	if err != nil {
		return fmt.Errorf("read known_hosts: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		HostKeyCallback: hostKeyCallback,

		User: login,
	}

	var pks []ssh.Signer
	socket, hasAgent := os.LookupEnv("SSH_AUTH_SOCK")
	if hasAgent {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			return fmt.Errorf("dial SSH_AUTH_SOCK: %w", err)
		}

		signers, err := agent.NewClient(conn).Signers()
		if err != nil {
			return fmt.Errorf("get signers from ssh-agent: %w", err)
		}

		if len(signers) > 0 {
			logger.Debug("found private keys via agent", zap.Int("keys", len(signers)))
		}

		pks = append(pks, signers...)
	}

	for _, key := range defaultKeys {
		keyPath := filepath.Join(osuser.HomeDir, ".ssh", key)
		content, err := os.ReadFile(keyPath)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.Error("failed to read key", zap.Error(err), zap.String("key", key))
			}

			continue
		}

		pk, err := ssh.ParsePrivateKey(content)
		if err != nil {
			logger.Error("failed to parse key", zap.Error(err), zap.String("key", key))
			continue
		}

		logger.Debug("found private key", zap.String("key", keyPath))
		pks = append(pks, pk)
	}

	if len(pks) > 0 {
		clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(pks...))
	}

	logger.Info("serving runtimes",
		zap.String("host", host),
		zap.String("port", port),
		zap.String("user", login))

	client := &runtimes.SSHClient{
		Hosts: []string{net.JoinHostPort(host, port)},
		User:  login,

		ClientConfig: clientConfig,
	}

	forwards := new(errgroup.Group)
	for _, runtime := range assoc {
		runtime := runtime
		forwards.Go(func() error {
			return client.Forward(ctx, runtime)
		})
	}

	return forwards.Wait()
}
