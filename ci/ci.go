package main

import (
	"dagger.io/dagger"
	"github.com/vito/bass/ci/pkgs"
)

func main() {
	dagger.ServeCommands(
		Build,
		Test,
		Generate,
	)
}

func Build(ctx dagger.Context, version string) (*dagger.Directory, error) {
	if version == "" {
		version = "dev"
	}

	return pkgs.GoBuild(ctx, Base(ctx), generate(ctx), pkgs.GoBuildOpts{
		Packages: []string{"./cmd/bass"},
		Xdefs:    []string{"main.version=" + version},
		Static:   true,
		Subdir:   "dist",
	}), nil
}

func Test(ctx dagger.Context) (string, error) {
	return pkgs.Gotestsum(ctx, Base(ctx), generate(ctx)).Stdout(ctx)
}

func Generate(ctx dagger.Context) (*dagger.Directory, error) {
	return generate(ctx), nil
}

func generate(ctx dagger.Context) *dagger.Directory {
	return pkgs.GoGenerate(ctx, Base(ctx), Code(ctx))
}

func Code(ctx dagger.Context) *dagger.Directory {
	return ctx.Client().Host().Directory(".", dagger.HostDirectoryOpts{
		Include: []string{
			"**/*.go",
			"**/go.mod",
			"**/go.sum",
			"**/testdata/**/*",
			"**/*.proto",
			"**/*.tmpl",
			"**/*.bass",
			"**/generate",
		},
		Exclude: []string{
			"ci/**/*",
		},
	})
}

func Base(ctx dagger.Context) *dagger.Container {
	return pkgs.Wolfi(ctx, []string{
		"go",
		"upx",
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"protobuf-dev",
	}).
		With(pkgs.GoCache(ctx)).
		With(pkgs.GoBin).
		WithExec([]string{"go", "install", "golang.org/x/tools/cmd/stringer@latest"}).
		WithExec([]string{"go", "install", "gotest.tools/gotestsum@latest"})
}
