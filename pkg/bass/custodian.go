package bass

import (
	"context"
	"io"

	"github.com/hashicorp/go-multierror"
)

type Custodian struct {
	closers []io.Closer
}

type custodianKey struct{}

func NewCustodian() *Custodian {
	return &Custodian{}
}

func WithCustodian(ctx context.Context, custodian *Custodian) context.Context {
	return context.WithValue(ctx, custodianKey{}, custodian)
}

func ForkCustodian(ctx context.Context) context.Context {
	child := NewCustodian()

	if parent, ok := CustodianFrom(ctx); ok {
		parent.AddCloser(child)
	}

	return context.WithValue(ctx, custodianKey{}, child)
}

func CustodianFrom(ctx context.Context) (*Custodian, bool) {
	custodian := ctx.Value(custodianKey{})
	if custodian != nil {
		return custodian.(*Custodian), true
	}

	return nil, false
}

func (c *Custodian) AddCloser(closer io.Closer) {
	c.closers = append(c.closers, closer)
}

func (c *Custodian) Close() error {
	var errs error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}
