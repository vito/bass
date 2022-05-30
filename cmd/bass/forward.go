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
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/ioctx"
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

func forwardLoop(ctx context.Context, sshAddr string, configs []bass.RuntimeConfig) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return withProgress(ctx, "forward", func(ctx context.Context, bassVertex *progrock.VertexRecorder) (err error) {
		exp := backoff.NewExponentialBackOff()
		exp.MaxElapsedTime = 0 // https://www.youtube.com/watch?v=6BtuqUX934U
		return backoff.Retry(func() error {
			return forward(ctx, sshAddr, configs)
		}, exp)
	})
}

func forward(ctx context.Context, sshAddr string, configs []bass.RuntimeConfig) error {
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

		logger.Debug("using private key", zap.String("key", keyPath))

		pks = append(pks, pk)
	}

	if len(pks) > 0 {
		clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(pks...))
	}

	logger.Info("forwarding runtimes",
		zap.String("host", host),
		zap.String("port", port),
		zap.String("user", login))

	client := &runtimes.SSHClient{
		Hosts: []string{net.JoinHostPort(host, port)},
		User:  login,

		ClientConfig: clientConfig,
	}

	recorder := progrock.RecorderFromContext(ctx)

	forwards := new(errgroup.Group)
	for _, runtime := range configs {
		runtime := runtime

		name := fmt.Sprintf("%s %s", runtime.Runtime, runtime.Platform)

		vtx := recorder.Vertex(digest.Digest("runtime:"+name), name)

		stderr := vtx.Stderr()

		// wire up logs to vertex
		logger := bass.LoggerTo(stderr).With(zap.String("side", "client"))
		ctx = zapctx.ToContext(ctx, logger)

		// wire up stderr for (log), (debug), etc.
		ctx = ioctx.StderrToContext(ctx, stderr)

		forwards.Go(func() error {
			err := client.Forward(ctx, runtime)
			vtx.Done(err)
			return err
		})
	}

	return forwards.Wait()
}
