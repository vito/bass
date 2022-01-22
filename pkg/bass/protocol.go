package bass

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// Protocol determines how response data is parsed from a thunk's response.
type Protocol interface {
	// DecodeInto decodes values from the reader and emits them to the sink.
	DecodeInto(context.Context, PipeSink, io.Reader) error
}

// WriteFlusher is a flushable io.Writer, to support protocols which have to
// maintain an internal buffer.
type WriteFlusher interface {
	io.Writer
	Flush() error
}

// Protocols defines the set of supported protocols for reading responses.
var Protocols = map[Symbol]Protocol{
	"raw":        RawProtocol{},
	"json":       JSONProtocol{},
	"unix-table": UnixTableProtocol{},
}

// DecodeProto uses the named protocol to decode values from r into the
// sink.
func DecodeProto(ctx context.Context, name Symbol, sink PipeSink, r io.Reader) error {
	proto, found := Protocols[name]
	if !found {
		return UnknownProtocolError{name}
	}

	return proto.DecodeInto(ctx, sink, r)
}

// UnknownProtocolError is returned when a thunk specifies an unknown
// response protocol.
type UnknownProtocolError struct {
	Protocol Symbol
}

func (err UnknownProtocolError) Error() string {
	return fmt.Sprintf("unknown protocol: %s", err.Protocol)
}

// UnixTableProtocol parses lines of tabular output with columns separated by
// whitespace.
//
// Each row is not guaranteed to have the same number of columns. Empty lines
// correspond to empty arrays.
type UnixTableProtocol struct{}

// DecodeInto decodes from r and emits lists of strings to the sink.
func (proto UnixTableProtocol) DecodeInto(ctx context.Context, sink PipeSink, r io.Reader) error {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		err := proto.emit(sink, strings.Fields(scanner.Text()))
		if err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (proto UnixTableProtocol) emit(w PipeSink, fields []string) error {
	strs := make([]Value, len(fields))
	for i := range fields {
		strs[i] = String(fields[i])
	}

	return w.Emit(NewList(strs...))
}

// JSON protocol decodes a values from JSON stream.
type JSONProtocol struct{}

var _ Protocol = JSONProtocol{}

// DecodeInto decodes a JSON stream from r and emits values to the sink.
func (JSONProtocol) DecodeInto(ctx context.Context, sink PipeSink, r io.Reader) error {
	src := NewJSONSource("internal", r)

	for {
		val, err := src.Next(ctx)
		if err != nil {
			if err == ErrEndOfSource {
				break
			}
			return err
		}

		err = sink.Emit(val)
		if err != nil {
			return err
		}
	}

	return nil
}

// Raw protocol buffers the entire stream and writes it as a single JSON string
// on flush.
type RawProtocol struct{}

var _ Protocol = RawProtocol{}

// DecodeInto reads the full content from r and emits it to the sink as
// a one big string.
func (RawProtocol) DecodeInto(ctx context.Context, sink PipeSink, r io.Reader) error {
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, r)
	if err != nil {
		return err
	}

	return sink.Emit(String(buf.String()))
}
