module github.com/vito/bass/pkg/runtimes/shim

go 1.17

// this file gets re-mounted as go.mod during shim building.
//
// it can't be committed as go.mod because that breaks go:embed.
//
// see golang/go#45197

require (
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/umoci v0.4.7
)

require (
	github.com/AdamKorcz/go-fuzz-headers v0.0.0-20210312213058-32f4d319f0d2 // indirect
	github.com/apex/log v1.4.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/klauspost/compress v1.11.3 // indirect
	github.com/klauspost/pgzip v1.2.4 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc90 // indirect
	github.com/opencontainers/runtime-spec v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rootless-containers/proto v0.1.0 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/urfave/cli v1.22.4 // indirect
	github.com/vbatts/go-mtree v0.5.0 // indirect
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9 // indirect
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4 // indirect
	google.golang.org/protobuf v1.24.0 // indirect
)
