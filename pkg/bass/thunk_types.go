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
	Platform   Platform `json:"platform"`
	Repository string   `json:"repository"`
	Tag        string   `json:"tag,omitempty"`
	Digest     string   `json:"digest,omitempty"`
}

func (ref ThunkImageRef) Ref() string {
	if ref.Digest != "" {
		return fmt.Sprintf("%s@%s", ref.Repository, ref.Digest)
	} else if ref.Tag != "" {
		return fmt.Sprintf("%s:%s", ref.Repository, ref.Tag)
	} else {
		return fmt.Sprintf("%s:latest", ref.Repository)
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
	Cache     *FileOrDirPath
	Secret    *Secret
}

var _ Decodable = &ThunkMountSource{}
var _ Encodable = ThunkMountSource{}

func (enum ThunkMountSource) ToValue() Value {
	if enum.HostPath != nil {
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
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return enum.FromValue(obj)
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

// ThunkImage specifies an OCI image, either by referencing a location or by
// referencing a path to an OCI image archive.
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
	} else {
		val, _ := ValueOf(*image.Thunk)
		return val
	}
}

func (image *ThunkImage) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return image.FromValue(obj)
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

func (path ThunkCmd) ToValue() Value {
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

func (path *ThunkCmd) UnmarshalJSON(payload []byte) error {
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return path.FromValue(obj)
}

func (path ThunkCmd) MarshalJSON() ([]byte, error) {
	return MarshalJSON(path.ToValue())
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
	var obj *Scope
	err := UnmarshalJSON(payload, &obj)
	if err != nil {
		return err
	}

	return path.FromValue(obj)
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
