package bass

import (
	"context"
	"fmt"
	"strings"
)

// Snitch reveals secrets.
type Snitch interface {
	// Ask fetches the secret value at the given path.
	//
	// By convention, the root of the path should be an applicable hostname,
	// followed by any further path qualifiers (perhaps a username), followed by
	// the name of a field.
	//
	// For example, the following secrets are all reasonable.
	//
	//    github.com/access_token  ; no username
	//    github.com/vito/password ; username in path
	Secret(FilePath) (Secret, error)

	// Fields fetches all fields of a secret at once, returning a Scope where
	// each value is a Secret.
	//
	// Use this when a credential is coming from an auto-generating backend and
	// multiple fields correspond to each other. For example, an AWS access key
	// and secret key.
	Fields(DirPath) (*Scope, error)
}

// SecretPath traverses the path or fetches the current value from its embedded
// Snitch.
type SecretPath struct {
	Path FileOrDirPath
}

var _ Value = SecretPath{}

func NewSecretPath(snitch Snitch, path FileOrDirPath) SecretPath {
	return SecretPath{
		Snitch: snitch,
		Path:   path,
	}
}

func (value SecretPath) String() string {
	return fmt.Sprintf("<secrets: %s>/%s", value.Snitch, strings.TrimPrefix(value.Path.Slash(), "./"))
}

func (value SecretPath) Equal(other Value) bool {
	var o SecretPath
	return other.Decode(&o) == nil &&
		value.Snitch == o.Snitch &&
		value.Path.FilesystemPath().Equal(o.Path.FilesystemPath())
}

func (value SecretPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *SecretPath:
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

func (path *SecretPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.SecretPath)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, path})
	}

	path.ID = p.Id

	return path.Path.UnmarshalProto(p.Path)
}

// Eval returns the value.
func (value SecretPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = ThunkPath{}

func (app SecretPath) Unwrap() Combiner {
	if app.Path.File != nil {
		return ThunkOperative{
			Cmd: ThunkCmd{
				Secret: &app,
			},
		}
	} else {
		return ExtendOperative{app}
	}
}

var _ Combiner = SecretPath{}

func (combiner SecretPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = SecretPath{}

func (path SecretPath) Name() string {
	// TODO: should this special-case ./ to return the thunk name?
	return path.Path.FilesystemPath().Name()
}

func (path SecretPath) Extend(ext Path) (Path, error) {
	extended := path

	var err error
	extended.Path, err = path.Path.Extend(ext)
	if err != nil {
		return nil, err
	}

	return extended, nil
}

func (value SecretPath) Dir() SecretPath {
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
