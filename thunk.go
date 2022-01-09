package bass

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/opencontainers/go-digest"
	"github.com/vito/invaders"
	"github.com/vito/progrock"
)

type Thunk struct {
	// Image specifies the OCI image in which to run the thunk.
	Image *ThunkImage `json:"image,omitempty"`

	// Insecure may be set to true to enable running the thunk with elevated
	// privileges. Its meaning is determined by the runtime.
	Insecure bool `json:"insecure,omitempty"`

	// Cmd identifies the file or command to run.
	Cmd ThunkCmd `json:"cmd"`

	// Args is a list of string or path arguments to pass to the command.
	Args []Value `json:"args,omitempty"`

	// Stdin is a list of arbitrary values, which may contain paths, to pass to
	// the command.
	Stdin []Value `json:"stdin,omitempty"`

	// Env is a mapping from environment variables to their string or path
	// values.
	Env *Scope `json:"env,omitempty"`

	// Dir configures a working directory in which to run the command.
	//
	// Note that a working directory is automatically provided to thunks by
	// the runtime. A relative Dir value will be relative to this working
	// directory, not the OCI image's initial working directory. The OCI image's
	// working directory is ignored.
	//
	// A relative directory path will be relative to the initial working
	// directory. An absolute path will be relative to the OCI image root.
	//
	// A thunk directory path may also be provided. It will be mounted to the
	// container and used as the working directory of the command.
	Dir *ThunkDir `json:"dir,omitempty"`

	// Mounts configures explicit mount points for the thunk, in addition to
	// any provided in Path, Args, Stdin, Env, or Dir.
	Mounts []ThunkMount `json:"mounts,omitempty"`

	// Response configures how a response may be fetched from the command.
	//
	// The Bass language expects responses to be in JSON stream format. From the
	// Runtime's perspective it may be arbitrary.
	Response ThunkResponse `json:"response,omitempty"`

	// Labels specify arbitrary fields for identifying the thunk, typically
	// used to influence caching behavior.
	//
	// For example, thunks which may return different results over time should
	// embed the current timestamp truncated to a certain amount of granularity,
	// e.g. one minute. Doing so prevents the first call from being cached
	// forever while still allowing some level of caching to take place.
	Labels *Scope `json:"labels,omitempty"`
}

func (wl Thunk) String() string {
	name, _ := wl.SHA1()
	return fmt.Sprintf("<thunk: %s>", name)
}

// SHA1 returns a stable SHA1 hash derived from the thunk.
func (wl Thunk) SHA1() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha1.Sum(payload)), nil
}

// SHA256 returns a stable SHA256 hash derived from the thunk.
func (wl Thunk) SHA256() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

// Avatar returns an ASCII art avatar derived from the thunk.
func (wl Thunk) Avatar() (*invaders.Invader, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return nil, err
	}

	h := fnv.New64a()
	_, err = h.Write(payload)
	if err != nil {
		return nil, err
	}

	invader := &invaders.Invader{}
	invader.Set(rand.New(rand.NewSource(int64(h.Sum64()))))
	return invader, nil
}

func (thunk Thunk) Vertex(recorder *progrock.Recorder) (*progrock.VertexRecorder, error) {
	sum, err := thunk.SHA256()
	if err != nil {
		panic(err)
	}

	dig := digest.NewDigestFromEncoded(digest.SHA256, sum)

	return recorder.Vertex(dig, fmt.Sprintf("thunk %s", dig)), nil
}

func (thunk *Thunk) UnmarshalJSON(b []byte) error {
	var obj *Scope
	err := json.Unmarshal(b, &obj)
	if err != nil {
		return err
	}

	return obj.Decode(thunk)
}

func (thunk *Thunk) Platform() *Platform {
	if thunk.Image == nil {
		return nil
	}

	return thunk.Image.Platform()
}
