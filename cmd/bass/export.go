package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/tonistiigi/units"
	"github.com/vito/bass"
	"github.com/vito/bass/runtimes"
	"github.com/vito/progrock"
)

var runExport bool

func init() {
	rootCmd.Flags().BoolVarP(&runExport, "export", "e", false, "write a workload path to stdout (directories are in tar format)")
}

func export(ctx context.Context, pool *runtimes.Pool) error {
	return withProgress(ctx, "export", func(ctx context.Context, vertex *progrock.VertexRecorder) error {
		dec := bass.NewDecoder(os.Stdin)

		var path bass.WorkloadPath
		err := dec.Decode(&path)
		if err != nil {
			bass.WriteError(ctx, err)
			return err
		}

		if isatty.IsTerminal(os.Stdout.Fd()) {
			r, w := io.Pipe()

			go func() {
				w.CloseWithError(pool.Export(ctx, w, path.Workload, path.Path.FilesystemPath()))
			}()

			return dumpTar(vertex.Stdout(), r)
		}

		return pool.Export(ctx, os.Stdout, path.Workload, path.Path.FilesystemPath())
	})
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
			fmt.Fprintf(w, "%s ", hdr.Typeflag)
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
