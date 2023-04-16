package bass

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vito/bass/pkg/internal"
)

// Protocol determines how response data is parsed from a thunk's response.
type Protocol interface {
	// DecodeStream decodes values from the reader.
	DecodeStream(context.Context, io.ReadCloser) (PipeSource, error)
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
	"lines":      LineProtocol{},
	"unix-table": UnixTableProtocol{},
	"tar":        TarProtocol{},
}

type TarProtocol struct{}

func (proto TarProtocol) DecodeStream(ctx context.Context, rc io.ReadCloser) (PipeSource, error) {
	return tarSource{
		tar.NewReader(rc),
		rc,
	}, nil
}

type tarSource struct {
	tr *tar.Reader
	io.Closer
}

func (src tarSource) String() string {
	return "<tar source>"
}

func (src tarSource) Next(ctx context.Context) (Value, error) {
	hdr, err := src.tr.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEndOfSource
		}

		return nil, err
	}

	var fsp FileOrDirPath
	if hdr.Typeflag == tar.TypeDir {
		dir := NewDirPath(hdr.Name)
		fsp.Dir = &dir
	} else {
		file := NewFilePath(hdr.Name)
		fsp.File = &file
	}

	meta := NewEmptyScope()
	meta.Set("type", String(hdr.Typeflag))
	meta.Set("name", String(hdr.Name))
	if hdr.Linkname != "" {
		meta.Set("link", String(hdr.Linkname))
	}
	meta.Set("size", Int(hdr.Size))
	meta.Set("mode", Int(hdr.Mode))
	meta.Set("uid", Int(hdr.Uid))
	meta.Set("gid", Int(hdr.Gid))
	if hdr.Uname != "" {
		meta.Set("uname", String(hdr.Uname))
	}
	if hdr.Gname != "" {
		meta.Set("gname", String(hdr.Gname))
	}

	return Annotated{
		Value: NewFSPath(internal.SingletonFS{
			Name:       hdr.Name,
			Info:       hdr.FileInfo(),
			ReadCloser: io.NopCloser(src.tr),
		}, fsp),
		Meta: meta,
	}, nil
}

// DecodeProto uses the named protocol to decode values from r into the
// sink.
func DecodeProto(ctx context.Context, name Symbol, r io.ReadCloser) (PipeSource, error) {
	proto, found := Protocols[name]
	if !found {
		return nil, UnknownProtocolError{name}
	}

	return proto.DecodeStream(ctx, r)
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
func (proto UnixTableProtocol) DecodeStream(ctx context.Context, rc io.ReadCloser) (PipeSource, error) {
	return unixTableSource{bufio.NewScanner(rc), rc}, nil
}

type unixTableSource struct {
	scanner *bufio.Scanner
	io.Closer
}

func (src unixTableSource) String() string {
	return "<unixTable source>"
}

func (src unixTableSource) Next(ctx context.Context) (Value, error) {
	if !src.scanner.Scan() {
		if src.scanner.Err() != nil {
			return nil, fmt.Errorf("next unix table row: %w", src.scanner.Err())
		}

		return nil, ErrEndOfSource
	}

	return src.toList(strings.Fields(src.scanner.Text())), nil
}

func (src unixTableSource) toList(fields []string) List {
	strs := make([]Value, len(fields))
	for i := range fields {
		strs[i] = String(fields[i])
	}

	return NewList(strs...)
}

// LineProtocol parse lines of output.
//
// Empty lines correspond to empty arrays.
type LineProtocol struct{}

// DecodeStream returns a pipe source decoding from r.
func (proto LineProtocol) DecodeStream(ctx context.Context, rc io.ReadCloser) (PipeSource, error) {
	return lineSource{bufio.NewScanner(rc), rc}, nil
}

type lineSource struct {
	scanner *bufio.Scanner
	io.Closer
}

func (src lineSource) String() string {
	return "<line source>"
}

func (src lineSource) Next(ctx context.Context) (Value, error) {
	if !src.scanner.Scan() {
		if src.scanner.Err() != nil {
			return nil, fmt.Errorf("next line: %w", src.scanner.Err())
		}

		return nil, ErrEndOfSource
	}

	return String(src.scanner.Text()), nil
}

// JSON protocol decodes a values from JSON stream.
type JSONProtocol struct{}

var _ Protocol = JSONProtocol{}

// DecodeStream returns a pipe source decoding from r.
func (JSONProtocol) DecodeStream(ctx context.Context, r io.ReadCloser) (PipeSource, error) {
	return NewJSONSource("internal", r), nil
}

// Raw protocol buffers the entire stream and writes it as a single JSON string
// on flush.
type RawProtocol struct{}

var _ Protocol = RawProtocol{}

// DecodeStream returns a pipe source decoding from r.
func (RawProtocol) DecodeStream(ctx context.Context, rc io.ReadCloser) (PipeSource, error) {
	return &rawSource{ReadCloser: rc}, nil
}

type rawSource struct {
	io.ReadCloser
	eos bool
}

func (src *rawSource) String() string {
	return "<raw source>"
}

func (src *rawSource) Next(ctx context.Context) (Value, error) {
	if src.eos {
		return nil, ErrEndOfSource
	}

	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, src)
	if err != nil {
		return nil, err
	}

	src.eos = true

	return String(buf.String()), nil
}
