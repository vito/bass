package main

import (
	"context"
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
	File *dagger.File
}

func (p *Pipeline) Build() *Dist {
	return &Dist{
		File: p.Bass.Build("dev").File("bass"),
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

func (p *Pipeline) Integration(
	ctx context.Context,
	unit *UnitTested,
) (*IntegrationTested, error) {
	out, err := p.Bass.Integration("", nil).CombinedOutput(ctx)
	if err != nil {
		return nil, err
	}
	return &IntegrationTested{
		Output: out,
	}, nil
}
