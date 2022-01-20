package bass_test

import (
	"testing"

	"github.com/vito/bass/bass"
	"github.com/vito/is"
)

func TestSecretEqual(t *testing.T) {
	secret := bass.NewSecret("token", []byte("x"))

	is := is.New(t)
	is.True(secret.Equal(secret))

	diffValue := bass.NewSecret("token", []byte("xy"))
	is.True(!secret.Equal(diffValue))

	// name doesn't matter
	diffName := bass.NewSecret("tolkein", []byte("x"))
	is.True(secret.Equal(diffName))
}

func TestSecretJSON(t *testing.T) {
	secret := bass.NewSecret("token", []byte("x"))

	is := is.New(t)
	is.True(secret.Equal(secret))

	payload, err := bass.MarshalJSON(secret)
	is.NoErr(err)
	is.Equal(string(payload), `{"secret":"token"}`+"\n")

	var unmarshaled bass.Secret
	err = bass.UnmarshalJSON(payload, &unmarshaled)
	is.NoErr(err)
	is.Equal(bass.NewSecret("token", nil), unmarshaled)
}
