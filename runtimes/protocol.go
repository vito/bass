package runtimes

import (
	"encoding/json"
	"fmt"
	"io"

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
