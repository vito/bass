package bass

import (
	"context"
	"io"
)

// Readable is any Value that can be (read).
type Readable interface {
	Value

	// CachePath returns a local file path to the content for caching purposes.
	//
	// Caches may be created under the given dest if needed. Implementations must
	// take care not to clobber each other's caches.
	CachePath(ctx context.Context, dest string) (string, error)

	// Open opens the resource for reading.
	Open(context.Context) (io.ReadCloser, error)
}

// Writable is any Value that can be the destination of (write).
type Writable interface {
	Value

	// Open opens the resource for reading.
	Write(context.Context, io.Reader) error
}
