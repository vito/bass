module github.com/vito/bass/pkg/runtimes/shim

// this file gets re-mounted as go.mod during shim building.
//
// it can't be committed as go.mod because that breaks go:embed.
//
// see golang/go#45197
