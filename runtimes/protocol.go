package runtimes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/vito/bass/ghcmd"
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
	"": JSONProtocol{}, // default

	"json":          JSONProtocol{},
	"github-action": GitHubActionProtocol{},
	"unix-table":    UnixTableProtocol{},
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
	return NopFlusher{res}
}

// NopFlusher is a no-op flushing Writer.
type NopFlusher struct {
	io.Writer
}

// Flush returns nil.
func (NopFlusher) Flush() error { return nil }

// GitHubActionProtocol handles a text response stream containing logs
// interspersed with GitHub workflow commands.
type GitHubActionProtocol struct{}

var _ Protocol = GitHubActionProtocol{}

// GitHubActionProtocol splits the stream into logs while interpreting workflow
// commands.
//
// Flush must be called in order to emit the finalized response.
func (GitHubActionProtocol) ResponseWriter(res io.Writer, logs LogWriter) ProtoWriter {
	enc := json.NewEncoder(res)

	ghaw := &gitHubActionWriter{
		Enc:      enc,
		Response: map[string]string{},
	}

	ghaw.Writer = &ghcmd.Writer{
		Writer:  logs,
		Handler: ghaw,
	}

	return ghaw
}

type gitHubActionWriter struct {
	*ghcmd.Writer

	Response map[string]string
	Enc      *json.Encoder
}

func (action *gitHubActionWriter) Flush() error {
	return action.Enc.Encode(action.Response)
}

func (action *gitHubActionWriter) HandleCommand(cmd *ghcmd.Command) error {
	switch cmd.Name {
	case "set-output":
		action.Response[cmd.Params["name"]] = cmd.Value
		return nil
	case "error":
		fmt.Fprintf(action.Writer, "\x1b[31merror: %s\x1b[0m\n", cmd.Value)
		return nil
	case "notice":
		fmt.Fprintf(action.Writer, "\x1b[34mnotice: %s\x1b[0m\n", cmd.Value)
		return nil
	case "warning":
		fmt.Fprintf(action.Writer, "\x1b[33mwarning: %s\x1b[0m\n", cmd.Value)
		return nil
	default:
		fmt.Fprintf(action.Writer, "\x1b[33munimplemented command: %s\x1b[0m\n", cmd)
		return nil
	}
}
