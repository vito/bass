package bass

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/proto"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
)

// A path created by a thunk.
type ThunkPath struct {
	Thunk Thunk         `json:"thunk"`
	Path  FileOrDirPath `json:"path"`
}

var _ Value = ThunkPath{}

// Hash returns a non-cryptographic hash derived from the thunk path.
func (thunkPath ThunkPath) Hash() (string, error) {
	msg, err := thunkPath.MarshalProto()
	if err != nil {
		return "", err
	}

	payload, err := gproto.Marshal(msg)
	if err != nil {
		return "", err
	}

	var sum [8]byte
	binary.BigEndian.PutUint64(sum[:], xxh3.Hash(payload))
	return base64.URLEncoding.EncodeToString(sum[:]), nil
}

func (value ThunkPath) String() string {
	return fmt.Sprintf("%s/%s", value.Thunk, strings.TrimPrefix(value.Path.Slash(), "./"))
}

func (value ThunkPath) Equal(other Value) bool {
	var o ThunkPath
	return other.Decode(&o) == nil &&
		value.Path.ToValue().Equal(o.Path.ToValue())
}

func (value *ThunkPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ThunkPath)
	if !ok {
		return DecodeError{msg, value}
	}

	if err := value.Thunk.UnmarshalProto(p.Thunk); err != nil {
		return err
	}

	if err := value.Path.UnmarshalProto(p.Path); err != nil {
		return err
	}

	return nil
}

func (value ThunkPath) MarshalJSON() ([]byte, error) {
	msg, err := value.MarshalProto()
	if err != nil {
		return nil, err
	}

	return protojson.Marshal(msg)
}

func (value *ThunkPath) UnmarshalJSON(b []byte) error {
	msg := &proto.ThunkPath{}
	err := protojson.Unmarshal(b, msg)
	if err != nil {
		return err
	}

	return value.UnmarshalProto(msg)
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

// Eval returns the value.
func (value ThunkPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app ThunkPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: ThunkCmd{
				Thunk: &app,
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
	// TODO: should this special-case ./ to return the thunk name?
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

func (path ThunkPath) CachePath(ctx context.Context, dest string) (string, error) {
	digest, err := path.Hash()
	if err != nil {
		return "", err
	}

	return Cache(ctx, filepath.Join(dest, "thunk-paths", digest), path)
}

func (path ThunkPath) Open(ctx context.Context) (io.ReadCloser, error) {
	platform := path.Thunk.Platform()
	if platform == nil {
		return nil, fmt.Errorf("cannot open bass thunk path: %s", path)
	}

	pool, err := RuntimeFromContext(ctx, *platform)
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
