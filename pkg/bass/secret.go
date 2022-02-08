package bass

import (
	"context"
	"crypto/subtle"
	"fmt"
)

var Secrets = NewEmptyScope()

func init() {
	Ground.Set("mask",
		Func("mask", "[secret name]", func(val String, name Symbol) Secret {
			return NewSecret(name, []byte(val))
		}),
		`shroud a string in secrecy`,
		`=> (mask "super secret" :github-token)`)
}

type Secret struct {
	Name Symbol `json:"secret"`

	// private to guard against accidentally revealing it when encoding to JSON
	// or something
	secret []byte
}

func NewSecret(name Symbol, inner []byte) Secret {
	return Secret{
		Name:   name,
		secret: inner,
	}
}

func (secret Secret) Reveal() []byte {
	return secret.secret
}

var _ Value = Secret{}

func (secret Secret) String() string {
	return fmt.Sprintf("<secret: %s (%d bytes)>", secret.Name, len(secret.secret))
}

// Eval does nothing and returns the secret.
func (secret Secret) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(secret, nil)
}

// Equal returns false; secrets cannot be compared.
func (secret Secret) Equal(other Value) bool {
	var o Secret
	return other.Decode(&o) == nil &&
		subtle.ConstantTimeCompare(secret.secret, o.secret) == 1
}

// Decode only supports decoding into a Secret or Value; it will not reveal the
// inner secret.
func (value Secret) Decode(dest interface{}) error {
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
