// Based on the TSA client from Concourse; see LICENSE.forward.go.md and
// NOTICE.forward.go.md.
//
// Adapted for use by Bass: lager -> zap, ATC/Garden/Baggageclaim -> Bass
// runtimes, removed the initialization pseudo-protocol and callback hooks.

package runtimes

import (
	"context"
	"encoding/hex"
	"errors"
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
	"github.com/vito/bass/pkg/proto"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

// RuntimeServiceName is the name given to the forwarded Unix socket that
// forwards the runtime GRPC service.
const RuntimeServiceName = "runtime"

// ErrKeepaliveTimeout is returned when the keepalive loop tries to send a
// keepalive request but takes too long, indicating a stuck/dead connection.
var ErrKeepaliveTimeout = errors.New("client->server keepalive ping timed out")

// SSHClient is a client for forwarding runtimes through a SSH gateway.
type SSHClient struct {
	Hosts  []string
	User   string
	Config *ssh.ClientConfig

	eg *errgroup.Group

	ssh  *ssh.Client
	conn *net.TCPConn
}

const heartbeatInterval = time.Minute
const heartbeatTimeout = 2 * time.Minute

// Dial establishes a connection to the remote SSH server and starts a
// keepalive loop monitoring the connection's health.
func (client *SSHClient) Dial(ctx context.Context) error {
	tcpConn, sshAddr, err := client.tryDialAll(ctx)
	if err != nil {
		return err
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(tcpConn, sshAddr, client.Config)
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	client.eg = eg

	sshClient := ssh.NewClient(clientConn, chans, reqs)
	client.ssh = sshClient
	client.conn = tcpConn.(*net.TCPConn)

	eg.Go(func() error {
		return client.keepAlive(ctx, heartbeatInterval, heartbeatTimeout)
	})

	return nil
}

// Close closes the internal SSH connection, if any exists.
func (client *SSHClient) Close(ctx context.Context) error {
	if client.ssh != nil {
		return client.ssh.Close()
	}

	return nil
}

// Wait waits for all forwarding sessions as well as the keepalive loop. If any
// of them fail, the rest will be interrupted, and Wait will return.
func (client *SSHClient) Wait() error {
	return client.eg.Wait()
}

// Forward opens a SSH tunnel forwarding traffic to/from the given runtime
// via gRPC.
func (client *SSHClient) Forward(ctx context.Context, assoc Assoc) error {
	logger := zapctx.FromContext(ctx)

	listener, err := client.ssh.Listen("unix", "/"+RuntimeServiceName)
	if err != nil {
		logger.Error("failed to listen", zap.Error(err))
		return err
	}

	srv := grpc.NewServer()
	proto.RegisterRuntimeServer(srv, &Server{Runtime: assoc.Runtime})

	client.eg.Go(func() error {
		if err := srv.Serve(listener); err != nil {
			logger.Error("failed to serve", zap.Error(err))
			return err
		}
		return nil
	})

	cmdline := []string{"forward"}
	if assoc.Platform.OS != "" {
		cmdline = append(cmdline, "--os", assoc.Platform.OS)
	} else {
		cmdline = append(cmdline, "--os", runtime.GOOS)
	}

	if assoc.Platform.Arch != "" {
		cmdline = append(cmdline, "--arch", assoc.Platform.Arch)
	} else {
		cmdline = append(cmdline, "--arch", runtime.GOARCH)
	}

	logger.Info("serving runtime",
		zap.Any("platform", assoc.Platform),
		zap.Strings("hosts", client.Hosts),
		zap.String("user", client.User))

	client.eg.Go(func() error {
		return client.run(ctx, strings.Join(cmdline, " "))
	})

	return nil
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

func (client *SSHClient) run(ctx context.Context, command string) (err error) {
	ctx, vtx := subVertex(ctx,
		digest.Digest(hex.EncodeToString(client.ssh.SessionID())),
		fmt.Sprintf("[ssh] %s", command),
	)
	defer func() { vtx.Done(err) }()

	logger := zapctx.FromContext(ctx).With(zap.String("side", "client"))

	sess, err := client.ssh.NewSession()
	if err != nil {
		logger.Error("failed to open session", zap.Error(err))
		return err
	}

	defer sess.Close()

	sess.Stdout = vtx.Stdout()
	sess.Stderr = vtx.Stderr()

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

func proxyListenerTo(ctx context.Context, listener net.Listener, network, addr string, service string, sessionID string) {
	conn := 0

	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			break
		}

		conn++

		subCtx, vtx := subVertex(ctx,
			digest.Digest(fmt.Sprintf("%s:service:%s:%d", sessionID, service, conn)),
			fmt.Sprintf("[ssh] [%s] conn:%d -> %s", service, conn, addr),
		)

		go func() {
			defer vtx.Complete()
			handleForwardedConn(subCtx, remoteConn, network, addr)
		}()
	}
}

func handleForwardedConn(ctx context.Context, remoteConn net.Conn, network, addr string) {
	defer remoteConn.Close()

	logger := zapctx.FromContext(ctx).With(zap.String("side", "client"))

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

func (client *SSHClient) keepAlive(ctx context.Context, interval time.Duration, timeout time.Duration) error {
	logger := zapctx.FromContext(ctx)

	keepAliveTicker := time.NewTicker(interval)

	// no matter how keepalive ends, clean up the connection state
	defer client.ssh.Close()

	for {
		sendKeepAliveRequest := make(chan error, 1)
		go func() {
			defer close(sendKeepAliveRequest)
			// ignore reply; server may just not have handled it, since there's no
			// standard keepalive request name
			_, _, err := client.ssh.Conn.SendRequest("keepalive", true, []byte("sup"))
			sendKeepAliveRequest <- err
		}()

		select {
		case <-time.After(timeout):
			logger.Error("timed out sending keepalive request")
			return ErrKeepaliveTimeout
		case err := <-sendKeepAliveRequest:
			if err != nil {
				logger.Error("failed sending keepalive request", zap.Error(err))
				return fmt.Errorf("send keepalive: %w", err)
			}
		}

		select {
		case <-keepAliveTicker.C:
			logger.Debug("keepalive")

		case <-ctx.Done():
			if err := client.conn.SetKeepAlive(false); err != nil {
				logger.Error("failed to disable keepalive", zap.Error(err))
				return fmt.Errorf("disable tcp keepalive: %w", err)
			}

			return ctx.Err()
		}
	}
}

func subVertex(ctx context.Context, id digest.Digest, name string) (context.Context, *progrock.VertexRecorder) {
	recorder := progrock.RecorderFromContext(ctx)

	vtx := recorder.Vertex(id, name)

	stderr := vtx.Stderr()

	// wire up logs to vertex
	level := zapctx.FromContext(ctx).Core()
	logger := bass.LoggerTo(stderr, level)
	ctx = zapctx.ToContext(ctx, logger)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	return ctx, vtx
}
