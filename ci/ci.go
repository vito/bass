package main

import (
	"context"
)

func main() {
	dag.Environment().
		WithArtifact(Build).
		WithCheck(Test).
		WithArtifact(Generate).
		WithShell(Dev).
		WithShell(Buildkitd).
		Serve()
}

func Build(version string) *Directory {
	if version == "" {
		version = "dev"
	}

	return dag.Go().Build(Base(), Generate(), GoBuildOpts{
		Packages: []string{"./cmd/bass"},
		Xdefs:    []string{"main.version=" + version},
		Static:   true,
		Subdir:   "dist",
	})
}

func Test(ctx context.Context) *EnvironmentCheck {
	return dag.Go().Test(
		Base().
			WithServiceBinding(
				"bass-buildkitd",
				Buildkitd().WithExec(nil, ContainerWithExecOpts{
					InsecureRootCapabilities: true,
				}),
			).
			WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234"),
		Generate(),
	)
}

func Buildkitd() *Container {
	return dag.Container().
		// TODO build instead
		From("basslang/buildkit:9b0bdb600641f3dd1d96f54ac2d86581ab6433b2").
		WithMountedCache("/var/lib/buildkit", dag.CacheVolume("bass-buildkitd4"), ContainerWithMountedCacheOpts{
			Sharing: Locked,
		}).
		WithEntrypoint([]string{
			"dumb-init",
			"buildkitd",
			"--debug",
			"--addr=tcp://0.0.0.0:1234",
		}).
		WithExposedPort(1234)
}

func Dev() *Container {
	dotNvim := dag.Git("https://github.com/vito/dot-nvim").Branch("main").Tree()
	return dag.Nix().
		Pkgs([]string{
			// for running scripts
			"bashInteractive",
			// go building + testing
			"go_1_20",
			"gcc",
			"gotestsum",
			// lsp tests
			"neovim",
			// packing bass.*.(tgz|zip)
			"gzip",
			"gnutar",
			"zip",
			// git plumbing
			"git",
			// compressing shim binaries
			"upx",
			// for sanity checking that upx exists
			//
			// not needed by nix, but needed by Makefile
			"which",
			// for building in test image
			"gnumake",
			// for protoc
			"protobuf",
			"protoc-gen-go",
			"protoc-gen-go-grpc",
			// docs
			"yarn",
		}).
		WithEnvVariable("HOME", "/root").
		WithExec([]string{"ln", "-sf", "/bin/nvim", "/bin/vim"}).
		WithMountedDirectory("/root/.config/nvim", dotNvim).
		WithFocus().
		WithExec([]string{
			"nvim", "-es",
			"-u", "/root/.config/nvim/plugins.vim",
			"-i", "NONE",
			"-c", "PlugInstall",
			"-c", "qa",
		}).
		With(dag.Go().GlobalCache).
		With(dag.Go().BinPath).
		WithExec([]string{"go", "install", "github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@latest"}).
		WithServiceBinding(
			"bass-buildkitd",
			Buildkitd().WithExec(nil, ContainerWithExecOpts{
				InsecureRootCapabilities: true,
			}),
		).
		WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234").
		WithMountedFile("/usr/bin/bass", Build("dev").File("dist/bass")).
		WithMountedDirectory("/src", Generate()).
		WithWorkdir("/src")
}

func Generate() *Directory {
	return dag.Go().Generate(Base(), Code())
}

func Code() *Directory {
	return dag.Host().Directory(".", HostDirectoryOpts{
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

func Base() *Container {
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
