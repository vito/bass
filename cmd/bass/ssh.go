package main

import (
	"context"
	"fmt"

	"github.com/gliderlabs/ssh"
	"github.com/google/go-github/v44/github"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap"
)

func sshBind(ctx context.Context) error {
	ssh.Handle(func(s ssh.Session) {
		fmt.Fprintf(s, "Hello, %s. Running %q.\n", s.User(), s.Command())
	})

	log := zapctx.FromContext(ctx)

	gh := github.NewClient(nil)

	opts := []ssh.Option{
		ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			keys, _, err := gh.Users.ListKeys(ctx, ctx.User(), nil)
			if err != nil {
				log.Error("failed to list keys", zap.Error(err), zap.String("user", ctx.User()))
				return false
			}

			for _, k := range keys {
				pkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.GetKey()))
				if err != nil {
					log.Error("failed to parse authorized key", zap.Error(err))
					return false
				}

				if ssh.KeysEqual(pkey, key) {
					log.Info("keys equal", zap.Int64("id", k.GetID()), zap.String("type", key.Type()))
					return true
				}
			}

			return false
		}),
	}

	if sshBindKey != "" {
		opts = append(opts, ssh.HostKeyFile(sshBindKey))
	}

	server := &ssh.Server{
		Addr: sshBindAddr,
	}

	for _, opt := range opts {
		server.SetOption(opt)
	}

	log.Info("ssh listening", zap.String("addr", sshBindAddr))

	return server.ListenAndServe()
}
