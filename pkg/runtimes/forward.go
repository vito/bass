// Based on the TSA client from Concourse; see LICENSE.forward.go.md and
// NOTICE.forward.go.md.
//
// Adapted for use by Bass: lager -> zap, ATC/Garden/Baggageclaim -> Bass
// runtimes, removed the initialization pseudo-protocol and callback hooks.

package runtimes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// SSHClient is a client for forwarding runtimes through a SSH gateway.
type SSHClient struct {
	Hosts []string
	User  string

	ClientConfig *ssh.ClientConfig
}

func (client *SSHClient) Forward(ctx context.Context, rc bass.RuntimeConfig) error {
	logger := zapctx.FromContext(ctx)

	sshClient, tcpConn, err := client.dial(ctx)
	if err != nil {
		logger.Error("failed to dial", zap.Error(err))
		return err
	}

	defer sshClient.Close()

	go keepAlive(ctx, sshClient, tcpConn, time.Minute, 5*time.Minute)

	for svc, netUrl := range rc.Addrs {
		logger := logger.With(zap.String("service", svc))

		listener, err := sshClient.Listen("unix", "/"+svc)
		if err != nil {
			logger.Error("failed to listen", zap.Error(err))
			return err
		}

		ctx := zapctx.ToContext(ctx, logger)
		switch netUrl.Scheme {
		case "unix":
			go proxyListenerTo(ctx, listener, "unix", netUrl.Path)
		case "tcp":
			go proxyListenerTo(ctx, listener, "tcp", netUrl.Host)
		}
	}

	cfg := bass.NewEmptyScope()
	if rc.Config != nil {
		cfg = rc.Config
	}

	config, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	cmdline := []string{"forward", "--runtime", rc.Runtime}
	if rc.Platform.OS != "" {
		cmdline = append(cmdline, "--os", rc.Platform.OS)
	} else {
		cmdline = append(cmdline, "--os", runtime.GOOS)
	}

	if rc.Platform.Arch != "" {
		cmdline = append(cmdline, "--arch", rc.Platform.Arch)
	} else {
		cmdline = append(cmdline, "--arch", runtime.GOARCH)
	}

	return client.run(
		ctx,
		sshClient,
		strings.Join(cmdline, " "),
		bytes.NewBuffer(config),
	)
}

func (client *SSHClient) dial(ctx context.Context) (*ssh.Client, *net.TCPConn, error) {
	tcpConn, sshAddr, err := client.tryDialAll(ctx)
	if err != nil {
		return nil, nil, err
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(tcpConn, sshAddr, client.ClientConfig)
	if err != nil {
		return nil, nil, err
	}

	return ssh.NewClient(clientConn, chans, reqs), tcpConn.(*net.TCPConn), nil
}

func (client *SSHClient) tryDialAll(ctx context.Context) (net.Conn, string, error) {
	logger := zapctx.FromContext(ctx)

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 15 * time.Second,
	}

	shuffled := make([]string, len(client.Hosts))
	copy(shuffled, client.Hosts)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	var errs error
	for _, host := range shuffled {
		conn, err := dialer.Dial("tcp", host)
		if err != nil {
			logger.Error("failed to connect", zap.Error(err))
			errs = multierror.Append(errs, err)
			continue
		}

		return conn, host, nil
	}

	return nil, "", errs
}

func (client *SSHClient) run(ctx context.Context, sshClient *ssh.Client, command string, stdin io.Reader) (err error) {
	recorder := progrock.RecorderFromContext(ctx)

	vtx := recorder.Vertex(
		digest.Digest(sshClient.SessionID()),
		fmt.Sprintf("[ssh] %s", command),
	)
	defer vtx.Done(err)

	stderr := vtx.Stderr()

	// wire up logs to vertex
	logger := bass.LoggerTo(stderr).With(zap.String("side", "client"))
	ctx = zapctx.ToContext(ctx, logger)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	sess, err := sshClient.NewSession()
	if err != nil {
		logger.Error("failed to open session", zap.Error(err))
		return err
	}

	defer sess.Close()

	sess.Stdin = stdin
	sess.Stdout = vtx.Stdout()
	sess.Stderr = stderr

	err = sess.Start(command)
	if err != nil {
		logger.Error("failed to start command", zap.Error(err))
		return err
	}

	errs := make(chan error, 1)
	go func() {
		errs <- sess.Wait()
	}()

	select {
	case <-ctx.Done():
		logger.Debug("stopping")
		return nil
	case err := <-errs:
		if err != nil {
			logger.Error("command failed", zap.Error(err))
			return err
		}

		logger.Debug("command exited")
		return nil
	}
}

func proxyListenerTo(ctx context.Context, listener net.Listener, network, addr string) {
	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			break
		}

		go handleForwardedConn(ctx, remoteConn, network, addr)
	}
}

func handleForwardedConn(ctx context.Context, remoteConn net.Conn, network, addr string) {
	logger := zapctx.FromContext(ctx).With(
		zap.String("process", "forwarded-conn"),
		zap.String("network", network),
		zap.String("addr", addr),
	)

	defer remoteConn.Close()

	var localConn net.Conn
	for {
		var err error
		localConn, err = net.Dial(network, addr)
		if err != nil {
			logger.Error("failed to dial", zap.Error(err))
			select {
			case <-ctx.Done():
				logger.Info("cancelled")
				return
			case <-time.After(1 * time.Second):
				logger.Info("retrying")
				continue
			}
		}

		break
	}
	defer localConn.Close()

	wg := new(sync.WaitGroup)

	pipe := func(to io.WriteCloser, from io.ReadCloser) {
		// if either end breaks, close both ends to ensure they're both unblocked,
		// otherwise io.Copy can block forever if e.g. reading after write end has
		// gone away
		defer to.Close()
		defer from.Close()
		defer wg.Done()

		io.Copy(to, from)
	}

	wg.Add(1)
	go pipe(localConn, remoteConn)

	wg.Add(1)
	go pipe(remoteConn, localConn)

	wg.Wait()
}

func keepAlive(ctx context.Context, sshClient *ssh.Client, tcpConn *net.TCPConn, interval time.Duration, timeout time.Duration) {
	logger := zapctx.FromContext(ctx)

	keepAliveTicker := time.NewTicker(interval)

	for {
		sendKeepAliveRequest := make(chan error, 1)
		go func() {
			defer close(sendKeepAliveRequest)
			// ignore reply; server may just not have handled it, since there's no
			// standard keepalive request name
			_, _, err := sshClient.Conn.SendRequest("keepalive", true, []byte("sup"))
			sendKeepAliveRequest <- err
		}()

		select {
		case <-time.After(timeout):
			logger.Error("timed out sending keepalive request")
			sshClient.Close()
			return
		case err := <-sendKeepAliveRequest:
			if err != nil {
				logger.Error("failed sending keepalive request", zap.Error(err))
				sshClient.Close()
				return
			}
		}

		select {
		case <-keepAliveTicker.C:
			logger.Debug("keepalive")

		case <-ctx.Done():
			if err := tcpConn.SetKeepAlive(false); err != nil {
				logger.Error("failed to disable keepalive", zap.Error(err))
				return
			}

			return
		}
	}
}
