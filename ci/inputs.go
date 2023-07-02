package main

import "dagger.io/dagger"

type Inputs struct {
	Nixpkgs *dagger.Directory
}

func LoadInputs(ctx dagger.Context) Inputs {
	c := ctx.Client()
	return Inputs{
		Nixpkgs: c.Git("https://github.com/nixos/nixpkgs").
			Branch("nixpkgs-unstable").
			Tree(),
	}
}
