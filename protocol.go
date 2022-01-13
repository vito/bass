package bass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Protocol determines how response data is parsed from a thunk's response.
type Protocol interface {
	// ResponseWriter constructs a ProtoWriter which will write a JSON stream of
	// response values to jsonW and write log output to logW.
	ResponseWriter(jsonW io.Writer, logW LogWriter) ProtoWriter
}

// LogWriter writes to the thunk's output log.
type LogWriter interface {
	io.Writer

	// TODO
	// Mask(string)
}

// ProtoWriter is a flushable io.Writer, to support protocols where a single
// response is created iteratively and needs to be emitted at the end.
type ProtoWriter interface {
	io.Writer
	Flush() error
}

// Protocols defines the set of supported protocols for reading responses.
var Protocols = map[string]Protocol{
	// "discard":    DiscardProtocol{},
	"raw":        RawProtocol{},
	"json":       JSONProtocol{},
	"unix-table": UnixTableProtocol{},
}

// UnknownProtocolError is returned when a thunk specifies an unknown
// response protocol.
type UnknownProtocolError struct {
	Protocol string
}

func (err UnknownProtocolError) Error() string {
	return fmt.Sprintf("unknown protocol: %s", err.Protocol)
}

// UnixTableProtocol parses lines of tabular output with columns separated by
// whitespace. It emits JSON encoded arrays containing each row's columns.
//
// Each row is not guaranteed to have the same number of columns. Empty lines
// correspond to empty arrays.
type UnixTableProtocol struct{}

// ResponseWriter returns res with a no-op Flush.
func (UnixTableProtocol) ResponseWriter(res io.Writer, _ LogWriter) ProtoWriter {
	return &unixTableWriter{
		enc: json.NewEncoder(res),
	}
}

type unixTableWriter struct {
	enc *json.Encoder
	buf []byte
}

func (w *unixTableWriter) Write(p []byte) (int, error) {
	written := len(p)

	for len(p) > 0 {
		if w.buf != nil {
			cp := []byte{}
			cp = append(cp, w.buf...)
			cp = append(cp, p...)
			p = cp
			w.buf = nil
		}

		ln := bytes.IndexRune(p, '\n')
		if ln == -1 {
			cp := []byte{}
			cp = append(cp, p...)
			w.buf = cp
			break
		}

		row := string(p[:ln])

		err := w.enc.Encode(strings.Fields(row))
		if err != nil {
			return 0, err
		}

		p = p[ln+1:]
	}

	return written, nil
}

func (w unixTableWriter) Flush() error {
	if len(w.buf) > 0 {
		return w.enc.Encode(strings.Fields(string(w.buf)))
	}

	return nil
}

// NewProtoWriter constructs a ProtoWriter for handling the named protocol.
func NewProtoWriter(name string, jsonW io.Writer, logW LogWriter) (ProtoWriter, error) {
	proto, found := Protocols[name]
	if !found {
		return nil, UnknownProtocolError{name}
	}

	return proto.ResponseWriter(jsonW, logW), nil
}

// JSON protocol is effectively a no-op, expecting a valid JSON stream to be
// returned in the response.
type JSONProtocol struct{}

var _ Protocol = JSONProtocol{}

// ResponseWriter returns res with a no-op Flush.
func (JSONProtocol) ResponseWriter(res io.Writer, _ LogWriter) ProtoWriter {
	return nopFlusher{res}
}

// NopFlusher is a no-op flushing Writer.
type nopFlusher struct {
	io.Writer
}

// Flush returns nil.
func (nopFlusher) Flush() error { return nil }

// Raw protocol buffers the entire stream and writes it as a single JSON string
// on flush.
type RawProtocol struct{}

var _ Protocol = RawProtocol{}

// ResponseWriter returns res with a no-op Flush.
func (RawProtocol) ResponseWriter(res io.Writer, _ LogWriter) ProtoWriter {
	return &rawWriter{
		enc: json.NewEncoder(res),
	}
}

type rawWriter struct {
	bytes.Buffer

	enc *json.Encoder
}

// Flush writes the buffered output string to the response stream.
func (w rawWriter) Flush() error {
	return w.enc.Encode(w.Buffer.String())
}
