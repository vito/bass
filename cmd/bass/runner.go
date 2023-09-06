package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/morikuni/aec"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/go-interact/interact"
	"github.com/vito/progrock"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

var defaultKeys = []string{
	"id_dsa",
	"id_ecdsa",
	"id_ecdsa_sk",
	"id_ed25519",
	"id_ed25519_sk",
	"id_rsa",
}

func runnerLoop(ctx context.Context, client *runtimes.SSHClient) error {
	ctx, pool, err := setupPool(ctx, false)
	if err != nil {
		return err
	}
	defer pool.Close()

	return cli.Step(ctx, cmdline, func(ctx context.Context, bassVertex *progrock.VertexRecorder) (err error) {
		exp := backoff.NewExponentialBackOff()
		exp.MaxElapsedTime = 0 // https://www.youtube.com/watch?v=6BtuqUX934U
		return backoff.Retry(func() error {
			return runner(ctx, client, pool.Runtimes)
		}, backoff.WithContext(exp, ctx))
	})
}

func knownHostsPrompter(knownHosts string) (ssh.HostKeyCallback, error) {
	check, err := knownhosts.New(knownHosts)
	if err != nil {
		return nil, fmt.Errorf("read known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := check(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			return handleKeyErr(keyErr, knownHosts, hostname, key)
		}

		return err
	}, nil
}

func handleKeyErr(keyErr *knownhosts.KeyError, knownHosts, hostname string, key ssh.PublicKey) error {
	if len(keyErr.Want) > 0 {
		// key mismatch (sketchy!)
		return keyErr
	}

	line := knownhosts.Line([]string{hostname}, key)

	fmt.Println("encountered unknown host key:")
	fmt.Println()
	fmt.Println("  " + aec.YellowF.Apply(line))
	fmt.Println()

	var add bool
	if err := interact.NewInteraction("do you trust this key?").Resolve(&add); err != nil {
		return err
	}

	if !add {
		return keyErr
	}

	return appendTo(knownHosts, line)
}

func appendTo(fp, line string) error {
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(line + "\n")
	return err
}

func runner(ctx context.Context, client *runtimes.SSHClient, assoc []runtimes.Assoc) error {
	err := client.Dial(ctx)
	if err != nil {
		return err
	}

	for _, runtime := range assoc {
		err := client.Forward(ctx, runtime)
		if err != nil {
			return err
		}
	}

	return client.Wait()
}

func runnerClient(ctx context.Context, sshAddr string) (*runtimes.SSHClient, error) {
	logger := zapctx.FromContext(ctx)

	osuser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
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

	hostKeyCallback, err := knownHostsPrompter(filepath.Join(osuser.HomeDir, ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("read known_hosts: %w", err)
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
			return nil, fmt.Errorf("dial SSH_AUTH_SOCK: %w", err)
		}

		signers, err := agent.NewClient(conn).Signers()
		if err != nil {
			return nil, fmt.Errorf("get signers from ssh-agent: %w", err)
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

	return &runtimes.SSHClient{
		Hosts: []string{net.JoinHostPort(host, port)},
		User:  login,

		Config: clientConfig,
	}, nil
}
