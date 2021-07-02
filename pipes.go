package bass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-colorable"
)

type PipeSource interface {
	String() string
	Next(context.Context) (Value, error)
	Close() error
}

type PipeSink interface {
	String() string
	Emit(Value) error
}

var Stdin = &Source{
	NewJSONSource("stdin", os.Stdin),
}

var Stdout = &Sink{
	NewJSONSink("stdout", os.Stdout),
}

var Stderr = colorable.NewColorableStderr()

type JSONSink struct {
	Name string

	enc *json.Encoder
}

var _ PipeSink = (*JSONSink)(nil)

func NewJSONSink(name string, out io.Writer) *JSONSink {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	return &JSONSink{
		Name: name,
		enc:  enc,
	}
}

func (sink *JSONSink) String() string {
	return sink.Name
}

func (sink *JSONSink) Emit(val Value) error {
	return sink.enc.Encode(val)
}

type JSONSource struct {
	Name string

	closer io.Closer
	dec    *json.Decoder
}

var _ PipeSource = (*JSONSource)(nil)

func NewJSONSource(name string, in io.Reader) *JSONSource {
	dec := json.NewDecoder(in)
	dec.UseNumber()

	closer, ok := in.(io.Closer)
	if !ok {
		closer = io.NopCloser(in)
	}

	return &JSONSource{
		Name: name,

		dec:    dec,
		closer: closer,
	}
}

func (source *JSONSource) String() string {
	return source.Name
}

func (source *JSONSource) Close() error {
	return source.closer.Close()
}

func (source *JSONSource) Next(context.Context) (Value, error) {
	var val interface{}
	err := source.dec.Decode(&val)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEndOfSource
		}

		return nil, err
	}

	return ValueOf(val)
}

type Sink struct {
	PipeSink PipeSink
}

var _ Value = (*Sink)(nil)

func (value *Sink) String() string {
	return fmt.Sprintf("<sink: %s>", value.PipeSink)
}

func (value *Sink) Eval(env *Env, cont Cont) (ReadyCont, error) {
	return cont.Call(value), nil
}

func (value *Sink) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Sink:
		*x = value
		return nil
	case *PipeSink:
		*x = value.PipeSink
		return nil
	default:
		return DecodeError{
			Destination: dest,
			Source:      value,
		}
	}
}

func (sink *Sink) Equal(other Value) bool {
	var o *Sink
	return other.Decode(&o) == nil && sink == o
}

type Source struct {
	PipeSource PipeSource
}

var _ Value = (*Source)(nil)

func (value *Source) String() string {
	return fmt.Sprintf("<source: %s>", value.PipeSource)
}

func (value *Source) Eval(env *Env, cont Cont) (ReadyCont, error) {
	return cont.Call(value), nil
}

func (value *Source) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Source:
		*x = value
		return nil
	case *PipeSource:
		*x = value.PipeSource
		return nil
	default:
		return DecodeError{
			Destination: dest,
			Source:      value,
		}
	}
}

func (value *Source) Equal(other Value) bool {
	var o *Source
	return other.Decode(&o) == nil && value == o
}
