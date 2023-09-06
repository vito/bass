package cli

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/adrg/xdg"
	"github.com/mattn/go-isatty"
	"github.com/opencontainers/go-digest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

var ProgressUI = progrock.DefaultUI()

func init() {
	rave := ui.NewRave()
	rave.AuthCallbackAddr = "localhost:6507"
	rave.SpotifyAuth = spotifyauth.New(
		spotifyauth.WithClientID("56f38795c77d45ee8d9db76a950258fc"),
		spotifyauth.WithRedirectURL(rave.SpotifyCallbackURL()),
		spotifyauth.WithScopes(spotifyauth.ScopeUserReadCurrentlyPlaying),
	)
	rave.SpotifyTokenPath, _ = xdg.ConfigFile("bass/auth/spotify.json")
	ProgressUI.Spinner = rave
}

type Progress struct {
	*progrock.Tape
}

type Vertex struct {
	*progrock.Vertex

	Log *bytes.Buffer
}

func NewProgress() *Progress {
	return &Progress{
		Tape: progrock.NewTape(),
	}
}

func (prog *Progress) Close() {}

func (prog *Progress) WrapError(msg string, err error) *ProgressError {
	return &ProgressError{
		msg:  msg,
		err:  err,
		prog: prog,
	}
}

func (prog *Progress) Summarize(w io.Writer) {
	prog.Render(w, ProgressUI)
}

var fancy bool

func init() {
	fancy = isatty.IsTerminal(os.Stdout.Fd()) ||
		isatty.IsTerminal(os.Stderr.Fd())

	if os.Getenv("BASS_FANCY_TUI") != "" {
		fancy = true
	} else if os.Getenv("BASS_SIMPLE_TUI") != "" {
		fancy = false
	}
}

func WithProgress(ctx context.Context, f func(context.Context) error) (err error) {
	tape, recorder, err := electRecorder()
	if err != nil {
		WriteError(ctx, err)
		return
	}

	ctx = progrock.ToContext(ctx, recorder)

	var stopRendering func()
	if tape != nil && fancy {
		defer cleanupRecorder()
		err = ProgressUI.Run(ctx, tape, func(ctx context.Context, ui progrock.UIClient) error {
			return f(ctx)
		})
	} else {
		err = f(ctx)
	}

	if stopRendering != nil {
		stopRendering()
	}

	if err != nil {
		WriteError(ctx, err)
	}

	return
}

func Step(ctx context.Context, name string, f func(context.Context, *progrock.VertexRecorder) error) error {
	recorder := progrock.FromContext(ctx)

	vtx := recorder.Vertex(digest.Digest(name), name)

	stderr := vtx.Stderr()

	// wire up logs to vertex
	level := zapctx.FromContext(ctx).Core()
	logger := bass.LoggerTo(stderr, level)
	ctx = zapctx.ToContext(ctx, logger)

	// wire up stderr for (log), (debug), etc.
	ctx = ioctx.StderrToContext(ctx, stderr)

	// run the task
	err := f(ctx, vtx)
	vtx.Done(err)
	return err
}
