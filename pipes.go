package bass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-colorable"
)

type PipeSource interface {
	String() string
	Next(context.Context) (Value, error)
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

type InMemorySink struct {
	Values []Value
}

func NewInMemorySink() *InMemorySink {
	return &InMemorySink{}
}

func (src *InMemorySink) String() string {
	vals := []string{}
	for _, val := range src.Values {
		vals = append(vals, val.String())
	}

	return strings.Join(vals, " ")
}

func (src *InMemorySink) Emit(val Value) error {
	src.Values = append(src.Values, val)
	return nil
}

func (sink *InMemorySink) Reset() {
	sink.Values = nil
}

func (sink *InMemorySink) Source() PipeSource {
	return NewInMemorySource(sink.Values...)
}

func NewJSONSink(name string, out io.Writer) *JSONSink {
	return &JSONSink{
		Name: name,
		enc:  NewEncoder(out),
	}
}

func (sink *JSONSink) String() string {
	return sink.Name
}

func (sink *JSONSink) Emit(val Value) error {
	return sink.enc.Encode(val)
}

type InMemorySource struct {
	vals   []Value
	offset int
}

var _ PipeSource = (*InMemorySource)(nil)

func NewInMemorySource(vals ...Value) *InMemorySource {
	return &InMemorySource{vals: vals}
}

func (src *InMemorySource) String() string {
	vals := []string{}
	for i, val := range src.vals {
		if i < src.offset {
			vals = append(vals, fmt.Sprintf("\x1b[2m%s\x1b[0m", val))
		} else if i == src.offset {
			vals = append(vals, fmt.Sprintf("\x1b[1m%s\x1b[0m", val))
		} else {
			vals = append(vals, val.String())
		}
	}

	return strings.Join(vals, " ")
}

func (src *InMemorySource) Next(ctx context.Context) (Value, error) {
	if src.offset >= len(src.vals) {
		return nil, ErrEndOfSource
	}

	val := src.vals[src.offset]
	src.offset++

	return val, nil
}

type JSONSource struct {
	Name string

	dec *json.Decoder
}

var _ PipeSource = (*JSONSource)(nil)

func NewJSONSource(name string, in io.Reader) *JSONSource {
	return &JSONSource{
		Name: name,

		dec: NewDecoder(in),
	}
}

func (source *JSONSource) String() string {
	return source.Name
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

func NewSink(ps PipeSink) *Sink {
	return &Sink{ps}
}

var _ Value = (*Sink)(nil)

func (value *Sink) String() string {
	return fmt.Sprintf("<sink: %s>", value.PipeSink)
}

func (value *Sink) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (value *Sink) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Sink:
		*x = value
		return nil
	case *Value:
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

func (value *Sink) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (sink *Sink) Equal(other Value) bool {
	var o *Sink
	return other.Decode(&o) == nil && sink == o
}

type Source struct {
	PipeSource PipeSource
}

func NewSource(ps PipeSource) *Source {
	return &Source{ps}
}

var _ Value = (*Source)(nil)

func (value *Source) String() string {
	return fmt.Sprintf("<source: %s>", value.PipeSource)
}

func (value *Source) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (value *Source) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Source:
		*x = value
		return nil
	case *Value:
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

func (value *Source) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

func (value *Source) Equal(other Value) bool {
	var o *Source
	return other.Decode(&o) == nil && value == o
}
