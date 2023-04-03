package bass

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/proto"
	"github.com/zeebo/xxh3"
)

// CachePath is a Path within an ephemeral directory managed by the runtime.
type CachePath struct {
	ID   string
	Path FileOrDirPath
}

var _ Value = CachePath{}

func NewCacheDir(id string) CachePath {
	return NewCachePath(id, ParseFileOrDirPath("."))
}

func NewCachePath(id string, path FileOrDirPath) CachePath {
	return CachePath{
		ID:   id,
		Path: path,
	}
}

func ParseCachePath(path string) CachePath {
	return NewCachePath(
		filepath.Dir(path),
		ParseFileOrDirPath(filepath.Base(path)),
	)
}

func (value CachePath) String() string {
	return fmt.Sprintf("<cache: %s>/%s", value.ID, strings.TrimPrefix(value.Path.Slash(), "./"))
}

// Hash returns a non-cryptographic hash of the cache path's ID.
func (value CachePath) Hash() string {
	return b32(xxh3.HashString(value.ID))
}

func (value CachePath) Equal(other Value) bool {
	var o CachePath
	return other.Decode(&o) == nil &&
		value.ID == o.ID &&
		value.Path.FilesystemPath().Equal(o.Path.FilesystemPath())
}

func (value CachePath) Decode(dest any) error {
	switch x := dest.(type) {
	case *CachePath:
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
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (path *CachePath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.CachePath)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, path})
	}

	path.ID = p.Id

	return path.Path.UnmarshalProto(p.Path)
}

// Eval returns the value.
func (value CachePath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app CachePath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: app,
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = CachePath{}

func (combiner CachePath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = CachePath{}

func (path CachePath) Name() string {
	// TODO: should this special-case ./ to return the thunk name?
	return path.Path.FilesystemPath().Name()
}

func (path CachePath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}

func (value CachePath) Dir() CachePath {
	cp := value

	if value.Path.Dir != nil {
		parent := value.Path.Dir.Dir()
		cp.Path = FileOrDirPath{Dir: &parent}
	} else {
		parent := value.Path.File.Dir()
		cp.Path = FileOrDirPath{Dir: &parent}
	}

	return cp
}
