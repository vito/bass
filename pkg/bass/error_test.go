package bass_test

import (
	"errors"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestErrorEqual(t *testing.T) {
	is := is.New(t)
	inner := errors.New("uh oh")
	err1 := bass.Error{Err: inner}
	err2 := bass.Error{Err: inner}
	is.True(err1.Equal(err2))

	other := errors.New("uh oh")
	err3 := bass.Error{Err: other}
	is.True(!err1.Equal(err3))
}

func TestErrorEval(t *testing.T) {
	is := is.New(t)
	errv := bass.Error{Err: errors.New("uh oh")}
	res, err := basstest.Eval(bass.NewEmptyScope(), errv)
	is.NoErr(err)
	is.Equal(errv, res)
}

func TestErrorCall(t *testing.T) {
	is := is.New(t)
	errv := bass.Error{Err: errors.New("uh oh")}
	res, err := basstest.Call(errv, bass.NewEmptyScope(), bass.Empty{})
	is.Equal(errv.Err, err)
	is.Equal(nil, res)
}

func TestErrorDecode(t *testing.T) {
	is := is.New(t)
	errv := bass.Error{Err: errors.New("uh oh")}

	var self bass.Value
	is.NoErr(errv.Decode(&self))
	is.Equal(errv, self)

	var other bass.Error
	is.NoErr(errv.Decode(&other))
	is.Equal(errv, other)

	var inner error
	is.NoErr(errv.Decode(&inner))
	is.Equal(errv.Err, inner)
}
