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

type StaticSource struct {
	vals   []Value
	offset int
}

var _ PipeSource = (*StaticSource)(nil)

func NewStaticSource(vals ...Value) PipeSource {
	return &StaticSource{vals: vals}
}

func (src *StaticSource) String() string {
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

func (src *StaticSource) Next(ctx context.Context) (Value, error) {
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
	dec := json.NewDecoder(in)
	dec.UseNumber()

	return &JSONSource{
		Name: name,

		dec: dec,
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

var _ Value = (*Sink)(nil)

func (value *Sink) String() string {
	return fmt.Sprintf("<sink: %s>", value.PipeSink)
}

func (value *Sink) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
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

var _ Value = (*Source)(nil)

func (value *Source) String() string {
	return fmt.Sprintf("<source: %s>", value.PipeSource)
}

func (value *Source) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
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
