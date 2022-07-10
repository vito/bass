package bass

import (
	"context"
	"fmt"

	"github.com/vito/bass/pkg/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

var Secrets = NewEmptyScope()

func init() {
	Ground.Set("mask",
		Func("mask", "[secret name]", func(val String, name Symbol) Secret {
			return NewSecret(name.String(), []byte(val))
		}),
		`shrouds a string in secrecy`,
		`Prevents the string from being revealed when the value is displayed.`,
		`Prevents the string from being revealed in a serialized thunk or thunk path.`,
		`Does NOT currently prevent the string's value from being displayed in log output; you still have to be careful there.`,
		`=> (mask "super secret" :github-token)`)
}

type Secret struct {
	// Path is the full path to a secret to fetch.
	//
	// By convention, the root of the path should be an applicable hostname,
	// followed by any further path qualifiers (perhaps a username), followed by
	// the name of a field.
	//
	// For example, the following secrets are all reasonable.
	//
	//    github.com/access_token  ; no username
	//    github.com/vito/password ; username in path
	Path FileOrDirPath

	// Prefetch is a prefix of the path containing multiple fields which should
	// be fetched all at once and used together within the scope of the secret's
	// use.
	//
	// Use this when a credential is coming from an auto-generating backend and
	// multiple fields correspond to each other. For example, an AWS access key
	// and secret key.
	Prefetch *DirPath
}

func NewSecret(path string) Secret {
	return Secret{
		Path: ParseFileOrDirPath(path),
	}
}

var _ Value = Secret{}

func (secret Secret) String() string {
	if secret.Prefetch != nil {
		return fmt.Sprintf("<secret: (%s)%s>", secret.Prefetch, secret.Path)
	}

	return fmt.Sprintf("<secret: %s>", secret.Path)
}

// Eval does nothing and returns the secret.
func (secret Secret) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(secret, nil)
}

// Equal returns false; secrets cannot be compared.
func (secret Secret) Equal(other Value) bool {
	var o Secret
	if other.Decode(&o) != nil {
		return false
	}

	if !secret.Path.ToValue().Equal(o.Path.ToValue()) {
		return false
	}

	if secret.Prefetch != nil && o.Prefetch != nil {
		return secret.Prefetch.Equal(o.Prefetch)
	}

	return secret.Prefetch == nil && o.Prefetch == nil
}

// Decode only supports decoding into a Secret or Value; it will not reveal the
// inner secret.
func (value Secret) Decode(dest any) error {
	switch x := dest.(type) {
	case *Secret:
		*x = value
		return nil
	case *Value:
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

func (value *Secret) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.Secret)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, value})
	}

	value.Name = p.Name

	return nil
}

func (value Secret) MarshalJSON() ([]byte, error) {
	msg, err := value.MarshalProto()
	if err != nil {
		return nil, err
	}

	return protojson.Marshal(msg)
}

func (value *Secret) UnmarshalJSON(b []byte) error {
	msg := &proto.Secret{}
	err := protojson.Unmarshal(b, msg)
	if err != nil {
		return err
	}

	return value.UnmarshalProto(msg)
}
