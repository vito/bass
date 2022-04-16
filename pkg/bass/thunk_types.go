package bass

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// ThunkMount configures a mount for the thunk.
type ThunkMount struct {
	Source ThunkMountSource `json:"source"`
	Target FileOrDirPath    `json:"target"`
}

// ThunkImageRef specifies an OCI image uploaded to a registry.
type ThunkImageRef struct {
	// The platform to target; influences runtime selection.
	Platform Platform `json:"platform"`

	// A reference to an image hosted on a registry.
	Repository string `json:"repository,omitempty"`

	// An OCI image archive tarball to load.
	File *ThunkPath `json:"file,omitempty"`

	// The tag to use, either from the repository or in a multi-tag OCI archive.
	Tag string `json:"tag,omitempty"`

	// An optional digest for maximally reprodicuble builds.
	Digest string `json:"digest,omitempty"`
}

func (ref ThunkImageRef) Ref() (string, error) {
	if ref.Repository == "" {
		return "", fmt.Errorf("ref does not refer to a repository: %s", ref)
	}

	if ref.Digest != "" {
		return fmt.Sprintf("%s@%s", ref.Repository, ref.Digest), nil
	} else if ref.Tag != "" {
		return fmt.Sprintf("%s:%s", ref.Repository, ref.Tag), nil
	} else {
		return fmt.Sprintf("%s:latest", ref.Repository), nil
	}
}

// Platform configures an OCI image platform.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch,omitempty"`
}

func (platform Platform) String() string {
	str := fmt.Sprintf("os=%s", platform.OS)
	if platform.Arch != "" {
		str += fmt.Sprintf(", arch=%s", platform.Arch)
	} else {
		str += ", arch=any"
	}
	return str
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

type ThunkMountSource struct {
	ThunkPath *ThunkPath
	HostPath  *HostPath
	FSPath    *FSPath
	Cache     *FileOrDirPath
	Secret    *Secret
}

var _ Decodable = &ThunkMountSource{}
var _ Encodable = ThunkMountSource{}

func (enum ThunkMountSource) ToValue() Value {
	if enum.FSPath != nil {
		val, _ := ValueOf(*enum.FSPath)
		return val
	} else if enum.HostPath != nil {
		val, _ := ValueOf(*enum.HostPath)
		return val
	} else if enum.Cache != nil {
		return enum.Cache.ToValue()
	} else if enum.Secret != nil {
		return *enum.Secret
	} else {
		val, _ := ValueOf(*enum.ThunkPath)
		return val
	}
}

func (enum *ThunkMountSource) UnmarshalJSON(payload []byte) error {
	return UnmarshalJSON(payload, enum)
}

func (enum ThunkMountSource) MarshalJSON() ([]byte, error) {
	return MarshalJSON(enum.ToValue())
}

func (enum *ThunkMountSource) FromValue(val Value) error {
	var host HostPath
	if err := val.Decode(&host); err == nil {
		enum.HostPath = &host
		return nil
	}

	var fs FSPath
	if err := val.Decode(&fs); err == nil {
		enum.FSPath = &fs
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

	var secret Secret
	if err := val.Decode(&secret); err == nil {
		enum.Secret = &secret
		return nil
	}

	return DecodeError{
		Source:      val,
		Destination: enum,
	}
}

// ThunkImage specifies the base image of a thunk - either a reference to be
// fetched, a thunk path (e.g. of a OCI/Docker tarball), or a lower thunk to
// run.
type ThunkImage struct {
	Ref   *ThunkImageRef
	Thunk *Thunk
}

func (img ThunkImage) Platform() *Platform {
	if img.Ref != nil {
		return &img.Ref.Platform
	} else {
		return img.Thunk.Platform()
	}
}

var _ Decodable = &ThunkImage{}
var _ Encodable = ThunkImage{}

func (image ThunkImage) ToValue() Value {
	if image.Ref != nil {
		val, _ := ValueOf(*image.Ref)
		return val
	} else if image.Thunk != nil {
		val, _ := ValueOf(*image.Thunk)
		return val
	} else {
		panic("empty ThunkImage or unhandled type?")
	}
}

func (image *ThunkImage) UnmarshalJSON(payload []byte) error {
	return UnmarshalJSON(payload, image)
}

func (image ThunkImage) MarshalJSON() ([]byte, error) {
	return MarshalJSON(image.ToValue())
}

func (image *ThunkImage) FromValue(val Value) error {
	var errs error

	var ref ThunkImageRef
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

type ThunkCmd struct {
	Cmd       *CommandPath
	File      *FilePath
	ThunkFile *ThunkPath
	Host      *HostPath
	FS        *FSPath
}

var _ Decodable = &ThunkCmd{}
var _ Encodable = ThunkCmd{}

func (cmd ThunkCmd) ToValue() Value {
	val, err := cmd.Inner()
	if err != nil {
		panic(err)
	}

	return val
}

func (cmd ThunkCmd) Inner() (Value, error) {
	if cmd.File != nil {
		return *cmd.File, nil
	} else if cmd.ThunkFile != nil {
		return *cmd.ThunkFile, nil
	} else if cmd.Cmd != nil {
		return *cmd.Cmd, nil
	} else if cmd.Host != nil {
		return *cmd.Host, nil
	} else if cmd.FS != nil {
		return *cmd.FS, nil
	} else {
		return nil, fmt.Errorf("no value present for thunk command: %+v", cmd)
	}
}

func (path *ThunkCmd) UnmarshalJSON(payload []byte) error {
	return UnmarshalJSON(payload, path)
}

func (path ThunkCmd) MarshalJSON() ([]byte, error) {
	val, err := path.Inner()
	if err != nil {
		return nil, err

	}
	return MarshalJSON(val)
}

func (path *ThunkCmd) FromValue(val Value) error {
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

type ThunkDir struct {
	Dir      *DirPath
	ThunkDir *ThunkPath
	HostDir  *HostPath
}

var _ Decodable = &ThunkDir{}
var _ Encodable = ThunkDir{}

func (path ThunkDir) ToValue() Value {
	if path.ThunkDir != nil {
		return *path.ThunkDir
	} else if path.Dir != nil {
		return *path.Dir
	} else {
		return *path.HostDir
	}
}

func (path *ThunkDir) UnmarshalJSON(payload []byte) error {
	return UnmarshalJSON(payload, path)
}

func (path ThunkDir) MarshalJSON() ([]byte, error) {
	return MarshalJSON(path.ToValue())
}

func (path *ThunkDir) FromValue(val Value) error {
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
