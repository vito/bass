// The Bass scripting language (https://bass-lang.org).
package main

import (
	"context"
	"main/internal/dagger"
)

type Bass struct {
	Src *dagger.Directory
}

func New(
	// +defaultPath="."
	src *dagger.Directory,
) *Bass {
	return &Bass{
		Src: src,
	}
}

func (b *Bass) Build(
	// +optional
	// +default="dev"
	version string,
	// +optional
	goos string,
	// +optional
	goarch string,
) *dagger.Directory {
	return dag.Go(dagger.GoOpts{
		Base: b.Base(),
	}).Build(b.Generate(), dagger.GoBuildOpts{
		Packages: []string{"./cmd/bass"},
		XDefs:    []string{"main.version=" + version},
		Static:   true,
		Goos:     goos,
		Goarch:   goarch,
	})
}

func (b *Bass) Repl() *dagger.Container {
	return dag.Apko().Wolfi([]string{"bash"}).
		WithFile("/usr/bin/bass", b.Build("dev", "", "").File("bass")).
		WithDefaultTerminalCmd([]string{"bash"}).
		// WithExec([]string{"bass"}, ContainerWithExecOpts{
		// 	ExperimentalPrivilegedNesting: true,
		// }).
		Terminal()
}

// +check
func (b *Bass) Unit(
	// +default=["./..."]
	// +optional
	packages []string,
	// +optional
	goTestFlags []string,
) *dagger.Container {
	return dag.Go(dagger.GoOpts{
		Base: b.Base(),
	}).Gotestsum(
		b.Generate(),
		dagger.GoGotestsumOpts{
			Packages:    packages,
			Nest:        true,
			GoTestFlags: append(goTestFlags, "-short"),
		})
}

// +check
func (b *Bass) Integration(
	// +optional
	// +default="Dagger"
	runtime string,
	// +optional
	goTestFlags []string,
) *dagger.Container {
	base := b.Base().
		WithFile("/usr/bin/bass", b.Build("dev", "", "").File("bass")) // for LSP tests
	if runtime != "" {
		goTestFlags = append(goTestFlags, "-run", "Runtimes/"+runtime)
	}
	if runtime != "Dagger" {
		// Dagger tests just use nesting, they don't need a buildkitd
		base = base.WithServiceBinding("bass-buildkitd", b.Buildkitd()).
			WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234")
	}
	return dag.Go(dagger.GoOpts{
		Base: base,
	}).Gotestsum(
		b.Generate(),
		dagger.GoGotestsumOpts{
			Packages:    []string{"./pkg/runtimes"},
			Nest:        true,
			GoTestFlags: goTestFlags,
		})
}

func (b *Bass) RunUnit(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Base: b.Base().
			WithServiceBinding("bass-buildkitd", b.Buildkitd()).
			WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234"),
	}).Gotestsum(b.Generate(), dagger.GoGotestsumOpts{
		Packages: []string{"./pkg/bass"},
		Nest:     true,
	}).Stdout(ctx)
}

func (b *Bass) Buildkitd() *dagger.Service {
	return dag.Container().
		// TODO build instead
		From("basslang/buildkit:9b0bdb600641f3dd1d96f54ac2d86581ab6433b2").
		WithMountedCache("/var/lib/buildkit", dag.CacheVolume("bass-buildkitd@2")).
		WithExposedPort(1234).
		WithDefaultArgs([]string{
			"dumb-init",
			"buildkitd",
			"--debug",
			"--addr=tcp://0.0.0.0:1234",
		}).
		AsService(dagger.ContainerAsServiceOpts{
			InsecureRootCapabilities: true,
		})
}

type Home interface {
	dagger.DaggerObject
	Install(base *dagger.Container) *dagger.Container
}

func (b *Bass) DevContainer(home Home /* +optional */) *dagger.Container {
	base := b.Base()
	if home != nil {
		base = home.Install(base)
	}
	return base.
		WithExec([]string{"go", "install", "github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@latest"}).
		WithServiceBinding("bass-buildkitd", b.Buildkitd()).
		WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234").
		WithFile("/usr/bin/bass", b.Build("dev", "", "").File("bass")).
		WithDirectory("/src", b.Src).
		WithWorkdir("/src").
		WithDefaultTerminalCmd([]string{"bash"})
}

func (b *Bass) Dev() *dagger.Container {
	return b.DevContainer(nil).Terminal()
}

func (b *Bass) Generate() *dagger.Directory {
	return dag.Go(dagger.GoOpts{Base: b.Base()}).Generate(b.Code())
}

func (b *Bass) Code() *dagger.Directory {
	return dag.Directory().WithDirectory(".", b.Src, dagger.DirectoryWithDirectoryOpts{
		Include: []string{
			".git",
			"**/*.go",
			"**/go.mod",
			"**/go.sum",
			"**/testdata/**/*",
			"**/*.proto",
			"**/*.tmpl",
			"**/*.bass",
			"**/*.lock",
			"**/generate",
			"**/Makefile",
		},
		Exclude: []string{
			"dagger/**/*",
		},
	})
}

func (b *Bass) Base() *dagger.Container {
	return dag.Apko().
		Wolfi([]string{
			"go-1.23",
			"protoc",
			"protoc-gen-go",
			"protoc-gen-go-grpc",
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
