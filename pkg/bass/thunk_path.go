package bass

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
)

// A path created by a thunk.
type ThunkPath struct {
	Thunk Thunk         `json:"thunk"`
	Path  FileOrDirPath `json:"path"`
}

var _ Value = ThunkPath{}

// SHA256 returns a stable SHA256 hash derived from the thunk path.
func (wl ThunkPath) SHA256() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

func (value ThunkPath) String() string {
	return fmt.Sprintf("%s/%s", value.Thunk, value.Path)
}

func (value ThunkPath) Equal(other Value) bool {
	var o ThunkPath
	return other.Decode(&o) == nil &&
		value.Path.ToValue().Equal(o.Path.ToValue())
}

func (value *ThunkPath) UnmarshalJSON(payload []byte) error {
	return UnmarshalJSON(payload, value)
}

func (value ThunkPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *ThunkPath:
		*x = value
		return nil
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Readable:
		*x = value
		return nil
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *ThunkPath) FromValue(val Value) error {
	var obj *Scope
	if err := val.Decode(&obj); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value ThunkPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app ThunkPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: ThunkCmd{
				ThunkFile: &app,
			},
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = ThunkPath{}

func (combiner ThunkPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = ThunkPath{}

func (path ThunkPath) Name() string {
	return path.Path.FilesystemPath().Name()
}

func (path ThunkPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}

func (path ThunkPath) Dir() ThunkPath {
	dirp := path.Path.File.Dir()
	path.Path = FileOrDirPath{Dir: &dirp}
	return path
}

var _ Readable = ThunkPath{}

func (path ThunkPath) Open(ctx context.Context) (io.ReadCloser, error) {
	pool, err := RuntimeFromContext(ctx, path.Thunk.Platform())
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	go func() {
		w.CloseWithError(pool.ExportPath(ctx, w, path))
	}()

	tr := tar.NewReader(r)

	_, err = tr.Next()
	if err != nil {
		return nil, err
	}

	return readCloser{tr, r}, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}
