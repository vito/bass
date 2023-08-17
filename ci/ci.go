package main

import (
	"dagger.io/dagger"
)

var dag = dagger.DefaultClient()

func main() {
	dag.Environment().
		WithArtifact_(Build).
		WithCheck_(Test).
		WithArtifact_(Generate).
		WithShell_(Repl)
}

func Build(ctx dagger.Context, version string) *dagger.Directory {
	if version == "" {
		version = "dev"
	}

	return dag.Go().Build(Base(), Generate(ctx), dagger.GoBuildOpts{
		Packages: []string{"./cmd/bass"},
		Xdefs:    []string{"main.version=" + version},
		Static:   true,
		Subdir:   "dist",
	})
}

func Test(ctx dagger.Context) (string, error) {
	return dag.Go().Gotestsum(Base(), Generate(ctx)).Stdout(ctx)
}

func Repl(ctx dagger.Context) *dagger.Container {
	return dag.Apko().Wolfi(nil).
		WithMountedFile("/usr/bin/bass", Build(ctx, "dev").File("dist/bass")).
		WithEntrypoint([]string{"/usr/bin/bass"})
}

func Generate(ctx dagger.Context) *dagger.Directory {
	return dag.Go().Generate(Base(), Code())
}

func Code() *dagger.Directory {
	return dag.Host().Directory(".", dagger.HostDirectoryOpts{
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

func Base() *dagger.Container {
	return dag.Apko().Wolfi([]string{
		"go",
		"upx",
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"protobuf-dev",
	}).
		With(dag.Go().GlobalCache).
		With(dag.Go().BinPath).
		WithExec([]string{"go", "install", "golang.org/x/tools/cmd/stringer@latest"}).
		WithExec([]string{"go", "install", "gotest.tools/gotestsum@latest"})
}
