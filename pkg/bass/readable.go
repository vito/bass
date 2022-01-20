package bass

import (
	"context"
	"io"
)

// Readable is any Value that can be (read).
type Readable interface {
	Value

	Open(context.Context) (io.ReadCloser, error)
}
