// The Bass scripting language (https://bass-lang.org).
package main

import (
	"context"
)

type Bass struct {
	Src *Directory
}

func New(src *Directory) *Bass {
	return &Bass{
		Src: src,
	}
}

func (b *Bass) Build(
	// +optional
	// +default="dev"
	version string,
) *Directory {
	return dag.Go(GoOpts{
		Base: b.Base(),
	}).Build(b.Generate(), GoBuildOpts{
		Packages: []string{"./cmd/bass"},
		XDefs:    []string{"main.version=" + version},
		Static:   true,
	})
}

func (b *Bass) Repl() *Terminal {
	return dag.Apko().Wolfi([]string{"bash"}).
		WithFile("/usr/bin/bass", b.Build("dev").File("bass")).
		WithDefaultTerminalCmd([]string{"bash"}).
		// WithExec([]string{"bass"}, ContainerWithExecOpts{
		// 	ExperimentalPrivilegedNesting: true,
		// }).
		Terminal()
}

func (b *Bass) Unit(
	// +default=["./..."]
	// +optional
	packages []string,
	// +optional
	goTestFlags []string,
) *Container {
	return dag.Go(GoOpts{
		Base: b.Base(),
	}).Gotestsum(
		b.Generate(),
		GoGotestsumOpts{
			Packages:    packages,
			Nest:        true,
			GoTestFlags: append(goTestFlags, "-short"),
		})
}

func (b *Bass) Integration(
	// +optional
	runtime string,
	// +optional
	goTestFlags []string,
) *Container {
	base := b.Base().
		WithFile("/usr/bin/bass", b.Build("dev").File("bass")) // for LSP tests
	if runtime != "" {
		goTestFlags = append(goTestFlags, "-run", runtime)
	}
	if runtime != "Dagger" {
		// Dagger tests just use nesting, they don't need a buildkitd
		base = base.WithServiceBinding("bass-buildkitd", b.Buildkitd()).
			WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234")
	}
	return dag.Go(GoOpts{
		Base: base,
	}).Gotestsum(
		b.Generate(),
		GoGotestsumOpts{
			Packages:    []string{"./pkg/runtimes"},
			Nest:        true,
			GoTestFlags: goTestFlags,
		})
}

func (b *Bass) RunUnit(ctx context.Context) (string, error) {
	return dag.Go(GoOpts{
		Base: b.Base().
			WithServiceBinding("bass-buildkitd", b.Buildkitd()).
			WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234"),
	}).Gotestsum(b.Generate(), GoGotestsumOpts{
		Packages: []string{"./pkg/bass"},
		Nest:     true,
	}).Stdout(ctx)
}

func (b *Bass) Buildkitd() *Service {
	return dag.Container().
		// TODO build instead
		From("basslang/buildkit:9b0bdb600641f3dd1d96f54ac2d86581ab6433b2").
		WithMountedCache("/var/lib/buildkit", dag.CacheVolume("bass-buildkitd@2")).
		WithEntrypoint([]string{
			"dumb-init",
			"buildkitd",
			"--debug",
			"--addr=tcp://0.0.0.0:1234",
		}).
		WithExposedPort(1234).
		WithExec(nil, ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		AsService()
}

type Home interface {
	DaggerObject
	Install(base *Container) *Container
}

func (b *Bass) DevContainer(home Home /* +optional */) *Container {
	base := b.Base()
	if home != nil {
		base = home.Install(base)
	}
	return base.
		WithExec([]string{"go", "install", "github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@latest"}).
		WithServiceBinding("bass-buildkitd", b.Buildkitd()).
		WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234").
		WithFile("/usr/bin/bass", b.Build("dev").File("bass")).
		WithDirectory("/src", b.Src).
		WithWorkdir("/src").
		WithDefaultTerminalCmd([]string{"bash"})
}

func (b *Bass) Dev() *Terminal {
	return b.DevContainer(nil).Terminal()
}

func (b *Bass) Generate() *Directory {
	return dag.Go(GoOpts{Base: b.Base()}).Generate(b.Code())
}

func (b *Bass) Code() *Directory {
	return dag.Directory().WithDirectory(".", b.Src, DirectoryWithDirectoryOpts{
		Include: []string{
			".git",
			"**/*.go",
			"**/go.mod",
			"**/go.sum",
			"**/testdata/**/*",
			"**/*.proto",
			"**/*.tmpl",
			"**/*.bass",
			"**/generate",
			"**/Makefile",
		},
		Exclude: []string{
			"dagger/**/*",
		},
	})
}

func (b *Bass) Base() *Container {
	return dag.Apko().
		Wolfi([]string{
			"go-1.22",
			"protoc",
			"protoc-gen-go",
			"protoc-gen-go-grpc=1.3.0-r12",
			"protobuf-dev",
			"git",    // basic plumbing
			"upx",    // compressing shim binaries
			"yarn",   // docs
			"neovim", // lsp tests
		}).
		With(dag.Go().GlobalCache).
		With(dag.Go().BinPath).
		WithExec([]string{"go", "install", "golang.org/x/tools/cmd/stringer@latest"}).
		WithExec([]string{"go", "install", "gotest.tools/gotestsum@latest"})
}
