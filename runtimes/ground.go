package runtimes

import (
	"archive/tar"
	"bytes"
	"context"
	"io"

	"github.com/vito/bass"
)

// init extends the Ground bass environment with additional core bindings for
// using runtimes.
func init() {
	bass.Ground.Set("load",
		bass.Func("load", "[thunk]", func(ctx context.Context, thunk bass.Thunk) (*bass.Scope, error) {
			runtime, err := RuntimeFromContext(ctx, thunk.Platform())
			if err != nil {
				return nil, err
			}

			return runtime.Load(ctx, thunk)
		}),
		`load a thunk into a bass.Ground`,
		`This is the primitive mechanism for loading other Bass code.`,
		`Typically used in combination with *dir* to load paths relative to the current file's directory.`)

	bass.Ground.Set("resolve",
		bass.Func("resolve", "[platform ref]", func(ctx context.Context, ref bass.ThunkImageRef) (bass.ThunkImageRef, error) {
			runtime, err := RuntimeFromContext(ctx, &ref.Platform)
			if err != nil {
				return bass.ThunkImageRef{}, err
			}

			return runtime.Resolve(ctx, ref)
		}),
		`load a thunk into a bass.Ground`,
		`This is the primitive mechanism for loading other Bass code.`,
		`Typically used in combination with *dir* to load paths relative to the current file's directory.`)

	bass.Ground.Set("run",
		bass.Func("run", "[thunk]", func(ctx context.Context, thunk bass.Thunk) (*bass.Source, error) {
			runtime, err := RuntimeFromContext(ctx, thunk.Platform())
			if err != nil {
				return nil, err
			}

			buf := new(bytes.Buffer)
			err = runtime.Run(ctx, buf, thunk)
			if err != nil {
				return nil, err
			}

			return bass.NewSource(bass.NewJSONSource(thunk.String(), buf)), nil
		}),
		`run a thunk`)

	bass.Ground.Set("read",
		bass.Func("read", "[thunk-path]", func(ctx context.Context, tp bass.ThunkPath) (string, error) {
			pool, err := RuntimeFromContext(ctx, tp.Thunk.Platform())
			if err != nil {
				return "", err
			}

			r, w := io.Pipe()

			go func() {
				w.CloseWithError(pool.ExportPath(ctx, w, tp))
			}()

			tr := tar.NewReader(r)

			_, err = tr.Next()
			if err != nil {
				return "", err
			}

			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, tr)
			if err != nil {
				return "", err
			}

			return buf.String(), nil
		}),
		`reads a thunk file path's content into a single string`,
		`See also (trim) for trimming whitespace from the content if desired.`)
}
