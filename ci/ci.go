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

func Test(ctx context.Context) *Container {
	return dag.Go().Gotestsum(Base(), Generate())
}

func Dev() *Container {
	dotNvim := dag.Git("https://github.com/vito/dot-nvim").Branch("main").Tree()
	return dag.Nix().Pkgs([]string{"bashInteractive", "neovim", "git"}).
		WithEnvVariable("HOME", "/root").
		WithExec([]string{"ln", "-sf", "/bin/nvim", "/bin/vim"}).
		WithMountedDirectory("/root/.config/nvim", dotNvim).
		WithFocus().
		WithExec([]string{
			"nvim",
			"-es",
			"-u",
			"/root/.config/nvim/plugins.vim",
			"-i", "NONE",
			"-c", "PlugInstall",
			"-c", "qa",
		}).
		WithMountedFile("/usr/bin/bass", Build("dev").File("dist/bass")).
		WithMountedDirectory("/src", Code()).
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
