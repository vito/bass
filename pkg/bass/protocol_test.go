package bass_test

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestProtocols(t *testing.T) {
	for _, e := range []BasicExample{
		{
			Name: "raw",
			Bass: `(take-all (read (mkfile ./foo "hello\nworld\n") :raw))`,
			Result: bass.NewList(
				bass.String("hello\nworld\n"),
			),
		},
		{
			Name:   "lines",
			Bass:   `(take-all (read (mkfile ./foo "hello\nworld\n") :lines))`,
			Result: bass.NewList(bass.String("hello"), bass.String("world")),
		},
		{
			Name:   "lines includes blank lines",
			Bass:   `(take-all (read (mkfile ./foo "hello\n\nworld\n") :lines))`,
			Result: bass.NewList(bass.String("hello"), bass.String(""), bass.String("world")),
		},
		{
			Name:   "lines includes last unterminated line",
			Bass:   `(take-all (read (mkfile ./foo "hello\nworld") :lines))`,
			Result: bass.NewList(bass.String("hello"), bass.String("world")),
		},
		{
			Name: "unix-table",
			Bass: `(take-all (read (mkfile ./foo "hello world\ngoodbye universe\n") :unix-table))`,
			Result: bass.NewList(
				bass.NewList(bass.String("hello"), bass.String("world")),
				bass.NewList(bass.String("goodbye"), bass.String("universe")),
			),
		},
		{
			Name: "unix-table includes empty rows",
			Bass: `(take-all (read (mkfile ./foo "hello world\n\ngoodbye universe\n") :unix-table))`,
			Result: bass.NewList(
				bass.NewList(bass.String("hello"), bass.String("world")),
				bass.NewList(),
				bass.NewList(bass.String("goodbye"), bass.String("universe")),
			),
		},
		{
			Name: "unix-table includes last unterminated row",
			Bass: `(take-all (read (mkfile ./foo "hello world\ngoodbye universe") :unix-table))`,
			Result: bass.NewList(
				bass.NewList(bass.String("hello"), bass.String("world")),
				bass.NewList(bass.String("goodbye"), bass.String("universe")),
			),
		},
		{
			Name: "json",
			Bass: `(take-all (read (mkfile ./foo "{\"a\":1}{\"b\":\"two\"}") :json))`,
			Result: bass.NewList(
				bass.Bindings{"a": bass.Int(1)}.Scope(),
				bass.Bindings{"b": bass.String("two")}.Scope(),
			),
		},
		{
			Name: "tar",
			Bass: `(collect (fn [f] {:meta (meta f) :content (-> f (read :raw) next)}) (read (mkfile ./foo tar-body) :tar))`,
			Bind: bass.Bindings{
				"tar-body": bass.String(tarBody(t,
					tarFile{
						header: &tar.Header{
							Name:     "some-file",
							Typeflag: tar.TypeReg,
							Mode:     0644,
							Uid:      1,
							Gid:      2,
							Uname:    "groot",
							Gname:    "rimz",
						},
						body: "some-content",
					},
					tarFile{
						header: &tar.Header{
							Name:     "some-dir",
							Typeflag: tar.TypeDir,
							Mode:     0755,
						},
					},
					tarFile{
						header: &tar.Header{
							Name:     "some-dir/sub-file",
							Typeflag: tar.TypeReg,
							Mode:     0755,
						},
						body: "sub-content",
					},
					tarFile{
						header: &tar.Header{
							Name:     "some-link",
							Linkname: "some-file",
							Typeflag: tar.TypeSymlink,
							Mode:     0777,
						},
					},
				)),
			},
			Result: bass.NewList(
				bass.Bindings{
					"meta": bass.Bindings{
						"name":  bass.String("some-file"),
						"type":  bass.String(tar.TypeReg),
						"size":  bass.Int(len("some-content")),
						"mode":  bass.Int(0644),
						"uid":   bass.Int(1),
						"gid":   bass.Int(2),
						"uname": bass.String("groot"),
						"gname": bass.String("rimz"),
					}.Scope(),
					"content": bass.String("some-content"),
				}.Scope(),
				bass.Bindings{
					"meta": bass.Bindings{
						"name": bass.String("some-dir"),
						"type": bass.String(tar.TypeDir),
						"size": bass.Int(0),
						"mode": bass.Int(0755),
						"uid":  bass.Int(0),
						"gid":  bass.Int(0),
					}.Scope(),
					"content": bass.String(""),
				}.Scope(),
				bass.Bindings{
					"meta": bass.Bindings{
						"name": bass.String("some-dir/sub-file"),
						"type": bass.String(tar.TypeReg),
						"size": bass.Int(len("sub-content")),
						"mode": bass.Int(0755),
						"uid":  bass.Int(0),
						"gid":  bass.Int(0),
					}.Scope(),
					"content": bass.String("sub-content"),
				}.Scope(),
				bass.Bindings{
					"meta": bass.Bindings{
						"name": bass.String("some-link"),
						"link": bass.String("some-file"),
						"type": bass.String(tar.TypeSymlink),
						"size": bass.Int(0),
						"mode": bass.Int(0777),
						"uid":  bass.Int(0),
						"gid":  bass.Int(0),
					}.Scope(),
					"content": bass.String(""),
				}.Scope(),
			),
		},
	} {
		e.Run(t)
	}
}

type tarFile struct {
	header *tar.Header
	body   string
}

func tarBody(t *testing.T, file ...tarFile) []byte {
	is := is.New(t)

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)

	for _, f := range file {
		f.header.Size = int64(len(f.body))

		is.NoErr(tw.WriteHeader(f.header))

		_, err := tw.Write([]byte(f.body))
		is.NoErr(err)

		is.NoErr(tw.Flush())
	}

	is.NoErr(tw.Close())

	return buf.Bytes()
}
