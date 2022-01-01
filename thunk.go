package bass

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/go-digest"
	"github.com/vito/invaders"
	"github.com/vito/progrock"
)

type Thunk struct {
	// Image specifies the OCI image in which to run the thunk.
	Image *ImageEnum `json:"image,omitempty"`

	// Insecure may be set to true to enable running the thunk with elevated
	// privileges. Its meaning is determined by the runtime.
	Insecure bool `json:"insecure,omitempty"`

	// Path identifies the file or command to run.
	Path RunPath `json:"path"`

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
	Dir *RunDirPath `json:"dir,omitempty"`

	// Mounts configures explicit mount points for the thunk, in addition to
	// any provided in Path, Args, Stdin, Env, or Dir.
	Mounts []RunMount `json:"mounts,omitempty"`

	// Response configures how a response may be fetched from the command.
	//
	// The Bass language expects responses to be in JSON stream format. From the
	// Runtime's perspective it may be arbitrary.
	Response Response `json:"response,omitempty"`

	// Labels specify arbitrary fields for identifying the thunk, typically
	// used to influence caching behavior.
	//
	// For example, thunks which may return different results over time should
	// embed the current timestamp truncated to a certain amount of granularity,
	// e.g. one minute. Doing so prevents the first call from being cached
	// forever while still allowing some level of caching to take place.
	Labels *Scope `json:"labels,omitempty"`
}

type RunMount struct {
	Source MountSourceEnum `json:"source"`
	Target FileOrDirPath   `json:"target"`
}

type MountSourceEnum struct {
	ThunkPath *ThunkPath
	HostPath  *HostPath
	Cache     *FileOrDirPath
}

var _ Decodable = &MountSourceEnum{}
var _ Encodable = MountSourceEnum{}

func (enum MountSourceEnum) ToValue() Value {
	if enum.HostPath != nil {
		val, _ := ValueOf(*enum.HostPath)
		return val
	} else if enum.Cache != nil {
		return enum.Cache.ToValue()
	} else {
		val, _ := ValueOf(*enum.ThunkPath)
		return val
	}
}

func (enum *MountSourceEnum) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return enum.FromValue(obj)
}

func (enum MountSourceEnum) MarshalJSON() ([]byte, error) {
	return MarshalJSON(enum.ToValue())
}

func (enum *MountSourceEnum) FromValue(val Value) error {
	var host HostPath
	if err := val.Decode(&host); err == nil {
		enum.HostPath = &host
		return nil
	}

	var tp ThunkPath
	if err := val.Decode(&tp); err == nil {
		enum.ThunkPath = &tp
		return nil
	}

	var cache FileOrDirPath
	if err := val.Decode(&cache); err == nil {
		enum.Cache = &cache
		return nil
	}

	return DecodeError{
		Source:      val,
		Destination: enum,
	}
}

// Response configures how a response may be fetched from the command.
type Response struct {
	// Stdout reads the response from the command's stdout stream.
	Stdout bool `json:"stdout,omitempty"`

	// File reads the response from the specified file in the container.
	File *FilePath `json:"file,omitempty"`

	// ExitCode converts the command's exit code into a response containing the
	// exit code number.
	ExitCode bool `json:"exit_code,omitempty"`

	// Protocol is the name of the protocol to use to read the response from the
	// specified location.
	//
	// Someday this may be able to point to a Bass script thunk path to run in
	// an isolated scope within the runtime. But for now it's just a string
	// protocol name and the runtimes just agree on their meaning and share code.
	//
	// If not specified, "json" stream is assumed.
	//
	// The given protocol is responsible for processing the output stream and
	// generating a cacheable response JSON stream. Any non-response content
	// (e.g. stdout logs interspersed with GitHub workflow commands) must also be
	// passed along live to the user, and cached interleaved with stderr output.
	//
	// Valid values: "json", "github-action", "unix-table"
	Protocol string `json:"protocol,omitempty"`
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

// ImageEnum specifies an OCI image, either by referencing a location or by
// referencing a path to an OCI image archive.
type ImageEnum struct {
	Ref   *ImageRef
	Thunk *Thunk
}

func (img ImageEnum) Platform() *Platform {
	if img.Ref != nil {
		return &img.Ref.Platform
	} else {
		return img.Thunk.Platform()
	}
}

// ImageRef specifies an OCI image uploaded to a registry.
type ImageRef struct {
	Platform   Platform `json:"platform"`
	Repository string   `json:"repository"`
	Tag        string   `json:"tag,omitempty"`
	Digest     string   `json:"digest,omitempty"`
}

// Platform configures an OCI image platform.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch,omitempty"`
}

// LinuxPlatform is the minimum configuration to select a Linux runtime.
var LinuxPlatform = Platform{
	OS: "linux",
}

// CanSelect returns true if the given platform (from a runtime) matches.
func (platform Platform) CanSelect(given Platform) bool {
	if platform.OS != given.OS {
		return false
	}

	return platform.Arch == "" || platform.Arch == given.Arch
}

var _ Decodable = &ImageEnum{}
var _ Encodable = ImageEnum{}

func (image ImageEnum) ToValue() Value {
	if image.Ref != nil {
		val, _ := ValueOf(*image.Ref)
		return val
	} else {
		val, _ := ValueOf(*image.Thunk)
		return val
	}
}

func (image *ImageEnum) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return image.FromValue(obj)
}

func (image ImageEnum) MarshalJSON() ([]byte, error) {
	return MarshalJSON(image.ToValue())
}

func (image *ImageEnum) FromValue(val Value) error {
	var errs error

	var ref ImageRef
	if err := val.Decode(&ref); err == nil {
		image.Ref = &ref
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", val, err))
	}

	var thunk Thunk
	if err := val.Decode(&thunk); err == nil {
		image.Thunk = &thunk
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", val, err))
	}

	return fmt.Errorf("image enum: %w", errs)
}

type RunPath struct {
	Cmd       *CommandPath
	File      *FilePath
	ThunkFile *ThunkPath
	Host      *HostPath
	FS        *FSPath
}

var _ Decodable = &RunPath{}
var _ Encodable = RunPath{}

func (path RunPath) ToValue() Value {
	if path.File != nil {
		return *path.File
	} else if path.ThunkFile != nil {
		return *path.ThunkFile
	} else if path.Cmd != nil {
		return *path.Cmd
	} else if path.Host != nil {
		return *path.Host
	} else if path.FS != nil {
		return *path.FS
	} else {
		panic("impossible: no value present for RunPath")
	}
}

func (path *RunPath) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return path.FromValue(obj)
}

func (path RunPath) MarshalJSON() ([]byte, error) {
	return MarshalJSON(path.ToValue())
}

func (path *RunPath) FromValue(val Value) error {
	var errs error
	var file FilePath
	if err := val.Decode(&file); err == nil {
		path.File = &file
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", file, err))
	}

	var cmd CommandPath
	if err := val.Decode(&cmd); err == nil {
		path.Cmd = &cmd
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", cmd, err))
	}

	var wlp ThunkPath
	if err := val.Decode(&wlp); err == nil {
		if wlp.Path.File != nil {
			path.ThunkFile = &wlp
			return nil
		} else {
			errs = multierror.Append(errs, fmt.Errorf("%T does not point to a File", wlp))
		}
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", wlp, err))
	}

	var host HostPath
	if err := val.Decode(&host); err == nil {
		path.Host = &host
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", file, err))
	}

	var fsp FSPath
	if err := val.Decode(&fsp); err == nil {
		path.FS = &fsp
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", file, err))
	}

	return errs
}

type RunDirPath struct {
	Dir      *DirPath
	ThunkDir *ThunkPath
	HostDir  *HostPath
}

var _ Decodable = &RunDirPath{}
var _ Encodable = RunDirPath{}

func (path RunDirPath) ToValue() Value {
	if path.ThunkDir != nil {
		return *path.ThunkDir
	} else if path.Dir != nil {
		return *path.Dir
	} else {
		return *path.HostDir
	}
}

func (path *RunDirPath) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return path.FromValue(obj)
}

func (path RunDirPath) MarshalJSON() ([]byte, error) {
	return MarshalJSON(path.ToValue())
}

func (path *RunDirPath) FromValue(val Value) error {
	var errs error

	var dir DirPath
	if err := val.Decode(&dir); err == nil {
		path.Dir = &dir
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", dir, err))
	}

	var wlp ThunkPath
	if err := val.Decode(&wlp); err == nil {
		if wlp.Path.Dir != nil {
			path.ThunkDir = &wlp
			return nil
		} else {
			return fmt.Errorf("dir thunk path must be a directory: %s", wlp)
		}
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", wlp, err))
	}

	var hp HostPath
	if err := val.Decode(&hp); err == nil {
		if hp.Path.Dir != nil {
			path.HostDir = &hp
			return nil
		} else {
			return fmt.Errorf("dir host path must be a directory: %s", wlp)
		}
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", hp, err))
	}

	return errs
}
