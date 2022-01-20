package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mattn/go-isatty"
	"github.com/tonistiigi/units"
	"github.com/vito/bass/bass"
	"github.com/vito/progrock"
)

var runExport bool

func init() {
	rootCmd.Flags().BoolVarP(&runExport, "export", "e", false, "write a thunk path to stdout as a tar stream, or log the tar contents if stdout is a tty")
}

func export(ctx context.Context) error {
	return withProgress(ctx, "export", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		dec := bass.NewDecoder(os.Stdin)

		var obj *bass.Scope
		err := dec.Decode(&obj)
		if err != nil {
			return err
		}

		var errs error

		var path bass.ThunkPath
		err = obj.Decode(&path)
		if err == nil {
			runtime, err := bass.RuntimeFromContext(ctx, path.Thunk.Platform())
			if err != nil {
				return err
			}

			return writeTar(vertex, func(w io.Writer) error {
				return runtime.ExportPath(ctx, w, path)
			})
		} else {
			errs = multierror.Append(errs, err)
		}

		var thunk bass.Thunk
		err = obj.Decode(&thunk)
		if err == nil {
			runtime, err := bass.RuntimeFromContext(ctx, thunk.Platform())
			if err != nil {
				return err
			}

			return writeTar(vertex, func(w io.Writer) error {
				return runtime.Export(ctx, w, thunk)
			})
		} else {
			errs = multierror.Append(errs, err)
		}

		return fmt.Errorf("unknown payload; must be a thunk or thunk path\n%w", errs)
	})
}

func writeTar(vertex *progrock.VertexRecorder, f func(w io.Writer) error) error {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		r, w := io.Pipe()

		go func() {
			w.CloseWithError(f(w))
		}()

		return dumpTar(vertex.Stdout(), r)
	}

	return f(os.Stdout)
}

func dumpTar(w io.Writer, r io.Reader) error {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		switch hdr.Typeflag {
		case tar.TypeReg:
			fmt.Fprintf(w, "f ")
		case tar.TypeLink:
			fmt.Fprintf(w, "l ")
		case tar.TypeSymlink:
			fmt.Fprintf(w, "s ")
		case tar.TypeChar:
			fmt.Fprintf(w, "c ")
		case tar.TypeBlock:
			fmt.Fprintf(w, "b ")
		case tar.TypeDir:
			fmt.Fprintf(w, "d ")
		case tar.TypeFifo:
			fmt.Fprintf(w, "f ")
		default:
			fmt.Fprintf(w, "%s ", string(hdr.Typeflag))
		}

		fmt.Fprintf(w,
			"%s\t%5.1f\t%s\t%s",
			os.FileMode(hdr.Mode),
			units.Bytes(hdr.Size),
			hdr.ModTime.Format(time.Stamp),
			hdr.Name,
		)

		if hdr.Linkname != "" {
			fmt.Fprintf(w, " -> %s", hdr.Linkname)
		}

		fmt.Fprintln(w)
	}

	return nil
}
