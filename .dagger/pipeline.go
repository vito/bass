package main

import (
	"context"
	"fmt"
	"main/internal/dagger"
)

func (b *Bass) Pipeline() *Pipeline {
	return &Pipeline{
		Bass: b,
	}
}

type Pipeline struct {
	// +private
	Bass *Bass
}

type Dist struct {
	LinuxAmd64   *dagger.File
	LinuxArm64   *dagger.File
	DarwinArm64  *dagger.File
	WindowsAmd64 *dagger.File
}

func (p *Pipeline) Build() *Dist {
	return &Dist{
		LinuxAmd64:   p.Bass.Build("dev", "linux", "amd64").File("bass"),
		LinuxArm64:   p.Bass.Build("dev", "linux", "arm64").File("bass"),
		DarwinArm64:  p.Bass.Build("dev", "darwin", "arm64").File("bass"),
		WindowsAmd64: p.Bass.Build("dev", "windows", "amd64").File("bass.exe"),
	}
}

type UnitTested struct {
	Output string
}

func (p *Pipeline) Unit(ctx context.Context) (*UnitTested, error) {
	out, err := p.Bass.Unit([]string{"./..."}, []string{"-race"}).CombinedOutput(ctx)
	if err != nil {
		return nil, err
	}
	return &UnitTested{
		Output: out,
	}, nil
}

type IntegrationTested struct {
	Output string
}

func (p *Pipeline) Integration(ctx context.Context) (*IntegrationTested, error) {
	out, err := p.Bass.Integration("", nil).CombinedOutput(ctx)
	if err != nil {
		return nil, err
	}
	return &IntegrationTested{
		Output: out,
	}, nil
}

func (p *Pipeline) Ship(
	ctx context.Context,
	dist *Dist,
	unit *UnitTested,
	integ *IntegrationTested,
	githubToken *dagger.Secret,
) error {
	fmt.Println("totally shipped it")
	return nil
}
