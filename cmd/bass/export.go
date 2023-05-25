package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mattn/go-isatty"
	"github.com/tonistiigi/units"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/progrock"
)

func export(ctx context.Context) error {
	ctx, _, err := setupPool(ctx, true)
	if err != nil {
		return err
	}

	return cli.Step(ctx, cmdline, func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		dec := bass.NewRawDecoder(os.Stdin)

		var msg json.RawMessage
		err := dec.Decode(&msg)
		if err != nil {
			return err
		}

		var errs error

		var path bass.ThunkPath
		err = json.Unmarshal([]byte(msg), &path)
		if err == nil {
			return exportPath(ctx, vertex, path)
		} else {
			errs = multierror.Append(errs, err)
		}

		var thunk bass.Thunk
		err = json.Unmarshal([]byte(msg), &thunk)
		if err == nil {
			return exportThunk(ctx, vertex, thunk)
		} else {
			errs = multierror.Append(errs, err)
		}

		return fmt.Errorf("unknown payload; must be a thunk or thunk path\n%w", errs)
	})
}

func exportPath(ctx context.Context, vertex *progrock.VertexRecorder, path bass.ThunkPath) error {
	platform := path.Thunk.Platform()
	if platform == nil {
		return fmt.Errorf("cannot export bass thunk path: %s", path)
	}

	runtime, err := bass.RuntimeFromContext(ctx, *platform)
	if err != nil {
		return err
	}

	return writeTar(vertex, func(w io.Writer) error {
		return runtime.ExportPath(ctx, w, path)
	})
}

func exportThunk(ctx context.Context, vertex *progrock.VertexRecorder, thunk bass.Thunk) error {
	platform := thunk.Platform()
	if platform == nil {
		return fmt.Errorf("cannot export bass thunk: %s", thunk)
	}

	runtime, err := bass.RuntimeFromContext(ctx, *platform)
	if err != nil {
		return err
	}

	return writeTar(vertex, func(w io.Writer) error {
		return runtime.Export(ctx, w, thunk)
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
