package bass

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-multierror"
)

type Workload struct {
	// Platform is an object used to select an appropriate runtime to run the
	// workload.
	//
	// Typical fields :os, :arch.
	Platform Object `json:"platform,omitempty"`

	// Image specifies the OCI image in which to run the workload.
	Image *ImageEnum `json:"image,omitempty"`

	// Insecure may be set to true to enable running the workload with elevated
	// privileges. Its meaning is determined by the runtime.
	Insecure bool `json:"insecure,omitempty"`

	// Entrypoint may be set to override an entrypoint configured in the OCI
	// image.
	Entrypoint []Value `json:"entrypoint,omitempty"`

	// Path identifies the file or command to run.
	Path RunPath `json:"path"`

	// Args is a list of string or path arguments to pass to the command.
	Args []Value `json:"args,omitempty"`

	// Stdin is a list of arbitrary values, which may contain paths, to pass to
	// the command.
	Stdin []Value `json:"stdin,omitempty"`

	// Env is a mapping from environment variables to their string or path
	// values.
	Env Object `json:"env,omitempty"`

	// Dir configures a working directory in which to run the command.
	//
	// Note that a working directory is automatically provided to workloads by
	// the runtime. A relative Dir value will be relative to this working
	// directory, not the OCI image's initial working directory. The OCI image's
	// working directory is ignored.
	//
	// A relative directory path will be relative to the initial working
	// directory. An absolute path will be relative to the OCI image root.
	//
	// A workload directory path may also be provided. It will be mounted to the
	// container and used as the working directory of the command.
	Dir *RunDirPath `json:"dir,omitempty"`

	// Mounts configures explicit mount points for the workload, in addition to
	// any provided in Path, Args, Stdin, Env, or Dir.
	Mounts []RunMount `json:"mounts,omitempty"`

	// Response configures how a response may be fetched from the command.
	//
	// The Bass language expects responses to be in JSON stream format. From the
	// Runtime's perspective it may be arbitrary.
	Response Response `json:"response,omitempty"`
}

type RunMount struct {
	Source WorkloadPath  `json:"source"`
	Target FileOrDirPath `json:"target"`
}

// ImageEnum specifies an OCI image, either by referencing a location or by
// referencing a path to an OCI image archive.
type ImageEnum struct {
	Ref  *ImageRef
	Path *WorkloadPath
}

// ImageRef specifies an OCI image uploaded to a registry.
type ImageRef struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag,omitempty"`
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
}

// SHA1 returns a stable SHA1 hash derived from the workload's content.
func (wl Workload) SHA1() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha1.Sum(payload)), nil
}

// SHA256 returns a stable SHA256 hash derived from the workload's content.
func (wl Workload) SHA256() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

func (wl *Workload) UnmarshalJSON(b []byte) error {
	var obj Object
	err := json.Unmarshal(b, &obj)
	if err != nil {
		return err
	}

	return obj.Decode(wl)
}

var _ Decodable = &ImageEnum{}
var _ Encodable = ImageEnum{}

func (image ImageEnum) ToValue() Value {
	if image.Ref != nil {
		val, _ := ValueOf(*image.Ref)
		return val
	} else {
		return *image.Path
	}
}

func (image *ImageEnum) UnmarshalJSON(payload []byte) error {
	var obj Object
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
	var ref ImageRef
	if err := val.Decode(&ref); err == nil {
		image.Ref = &ref
		return nil
	}

	var path WorkloadPath
	if err := val.Decode(&path); err == nil {
		image.Path = &path
		return nil
	}

	return DecodeError{
		Source:      val,
		Destination: image,
	}
}

type RunPath struct {
	Cmd          *CommandPath
	File         *FilePath
	WorkloadFile *WorkloadPath
}

var _ Decodable = &RunPath{}
var _ Encodable = RunPath{}

func (path RunPath) ToValue() Value {
	if path.File != nil {
		return *path.File
	} else if path.WorkloadFile != nil {
		return *path.WorkloadFile
	} else {
		return *path.Cmd
	}
}

func (path *RunPath) UnmarshalJSON(payload []byte) error {
	var obj Object
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

	var wlp WorkloadPath
	if err := val.Decode(&wlp); err == nil {
		if wlp.Path.File != nil {
			path.WorkloadFile = &wlp
			return nil
		} else {
			errs = multierror.Append(errs, fmt.Errorf("%T does not point to a File", wlp))
		}
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", wlp, err))
	}

	return errs
}

type RunDirPath struct {
	Dir         *DirPath
	WorkloadDir *WorkloadPath
}

var _ Decodable = &RunDirPath{}
var _ Encodable = RunDirPath{}

func (path RunDirPath) ToValue() Value {
	if path.WorkloadDir != nil {
		return *path.WorkloadDir
	} else {
		return *path.Dir
	}
}

func (path *RunDirPath) UnmarshalJSON(payload []byte) error {
	var obj Object
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

	var wlp WorkloadPath
	if err := val.Decode(&wlp); err == nil {
		if wlp.Path.Dir != nil {
			path.WorkloadDir = &wlp
			return nil
		} else {
			return fmt.Errorf("dir workload path must be a directory: %s", wlp)
		}
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", wlp, err))
	}

	return errs
}
