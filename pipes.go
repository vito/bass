package bass

import (
	"context"
	"encoding/json"
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
	&JSONSource{
		Name:    "stdin",
		Closer:  os.Stdin,
		Decoder: json.NewDecoder(os.Stdin),
	},
}

var Stdout = &Sink{
	&JSONSink{
		Name:    "stdout",
		Encoder: json.NewEncoder(os.Stdout),
	},
}

var Stderr = colorable.NewColorableStderr()

type JSONSink struct {
	Name string

	Encoder *json.Encoder
}

var _ PipeSink = (*JSONSink)(nil)

func (sink *JSONSink) String() string {
	return sink.Name
}

func (sink *JSONSink) Emit(val Value) error {
	return sink.Encoder.Encode(val)
}

type JSONSource struct {
	Name string

	io.Closer
	Decoder *json.Decoder
}

var _ PipeSource = (*JSONSource)(nil)

func (source *JSONSource) String() string {
	return source.Name
}

func (source *JSONSource) Next(context.Context) (Value, error) {
	var val interface{}
	err := source.Decoder.Decode(&val)
	if err != nil {
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

func (value *Sink) Eval(*Env) (Value, error) {
	return value, nil
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

func (value *Source) Eval(*Env) (Value, error) {
	return value, nil
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
