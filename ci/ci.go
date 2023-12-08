package main

import (
	"context"
	"path/filepath"
)

type Bass struct {
}

func (b *Bass) Build(version string) *Directory {
	if version == "" {
		version = "dev"
	}

	return dag.Go(GoOpts{
		Base: b.Base(),
	}).Build(b.Generate(), GoBuildOpts{
		Packages: []string{"./cmd/bass"},
		XDefs:    []string{"main.version=" + version},
		Static:   true,
		Subdir:   "dist",
	})
}

func (b *Bass) Test(ctx context.Context) *Container {
	return dag.Go(GoOpts{
		Base: b.Base().
			WithServiceBinding(
				"bass-buildkitd",
				b.Buildkitd().WithExec(nil, ContainerWithExecOpts{
					InsecureRootCapabilities: true,
				}).AsService(),
			).
			WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234"),
	}).Gotestsum(b.Generate(), GoGotestsumOpts{
		Nest: true,
	})
}

func (b *Bass) Buildkitd() *Container {
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

//func (b *Bass) Dev() *Container {
//	dotNvim := dag.Git("https://github.com/vito/dot-nvim").Branch("main").Tree()
//	return dag.Nix().
//		Pkgs([]string{
//			// for running scripts
//			"bashInteractive",
//			// go building + testing
//			"go_1_20",
//			"gcc",
//			"gotestsum",
//			// lsp tests
//			"neovim",
//			// packing bass.*.(tgz|zip)
//			"gzip",
//			"gnutar",
//			"zip",
//			// git plumbing
//			"git",
//			// compressing shim binaries
//			"upx",
//			// for sanity checking that upx exists
//			//
//			// not needed by nix, but needed by Makefile
//			"which",
//			// for building in test image
//			"gnumake",
//			// for protoc
//			"protobuf",
//			"protoc-gen-go",
//			"protoc-gen-go-grpc",
//			// docs
//			"yarn",
//			// misc
//			"htop",
//		}).
//		WithEnvVariable("HOME", "/root").
//		WithExec([]string{"ln", "-sf", "/bin/nvim", "/bin/vim"}).
//		WithMountedDirectory("/root/.config/nvim", dotNvim).
//		WithFocus().
//		WithExec([]string{
//			"nvim", "-es",
//			"-u", "/root/.config/nvim/plugins.vim",
//			"-i", "NONE",
//			"-c", "PlugInstall",
//			"-c", "qa",
//		}).
//		With(dag.Go().GlobalCache).
//		With(dag.Go().BinPath).
//		WithExec([]string{"go", "install", "github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@latest"}).
//		WithServiceBinding(
//			"bass-buildkitd",
//			b.Buildkitd().WithExec(nil, ContainerWithExecOpts{
//				InsecureRootCapabilities: true,
//			}),
//		).
//		WithEnvVariable("BUILDKIT_HOST", "tcp://bass-buildkitd:1234").
//		WithMountedFile("/usr/bin/bass", b.Build("dev").File("dist/bass")).
//		WithMountedDirectory("/src", b.Generate()).
//		WithWorkdir("/src")
//}

func (b *Bass) Generate() *Directory {
	return dag.Go(GoOpts{Base: b.Base()}).Generate(b.Code())
}

func (b *Bass) Code() *Directory {
	root, err := filepath.Abs("..")
	if err != nil {
		panic(err)
	}
	return dag.Host().Directory(root, HostDirectoryOpts{
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

func (b *Bass) Base() *Container {
	return dag.Apko().Wolfi([]string{
		"go-1.21",
		"upx",
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"protobuf-dev",
	}).
		With(dag.Go().GlobalCache).
		With(dag.Go().BinPath).
		WithExec([]string{"go", "install", "golang.org/x/tools/cmd/stringer@latest"}).
		WithExec([]string{"go", "install", "gotest.tools/gotestsum@latest"}).
		WithFocus()
}
