package bass

import (
	"context"
	"io"
)

// Readable is any Value that can be (read).
type Readable interface {
	Value

	ReadAll(context.Context, io.Writer) error
}
