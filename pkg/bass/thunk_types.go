package bass

import (
	"fmt"
	"runtime"

	"github.com/containerd/containerd/platforms"
	"github.com/hashicorp/go-multierror"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vito/bass/pkg/proto"
)

// ThunkMount configures a mount for the thunk.
type ThunkMount struct {
	Source ThunkMountSource `json:"source"`
	Target FileOrDirPath    `json:"target"`
}

func (mount *ThunkMount) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ThunkMount)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, mount})
	}

	if err := mount.Source.UnmarshalProto(p.GetSource()); err != nil {
		return fmt.Errorf("unmarshal proto source: %w", err)
	}

	if err := mount.Target.UnmarshalProto(p.GetTarget()); err != nil {
		return fmt.Errorf("unmarshal proto target: %w", err)
	}

	return nil
}

func (mount ThunkMount) MarshalProto() (proto.Message, error) {
	tm := &proto.ThunkMount{}

	src, err := mount.Source.MarshalProto()
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	tm.Source = src.(*proto.ThunkMountSource)

	tgt, err := mount.Target.MarshalProto()
	if err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}

	tm.Target = tgt.(*proto.FilesystemPath)

	return tm, nil
}

// ImageRef specifies an OCI image uploaded to a registry.
type ImageRef struct {
	// A reference to an image hosted on a registry.
	Repository ImageRepository `json:"repository"`

	// The platform to target; influences runtime selection.
	Platform Platform `json:"platform,omitempty"`

	// The tag to use, either from the repository or in a multi-tag OCI archive.
	Tag string `json:"tag,omitempty"`

	// An optional digest for maximally reprodicuble builds.
	Digest string `json:"digest,omitempty"`
}

func (ref ImageRef) Ref() (string, error) {
	if ref.Repository.Static == "" {
		return "", fmt.Errorf("ref does not refer to a static repository")
	}

	repo := ref.Repository.Static

	if ref.Digest != "" {
		return fmt.Sprintf("%s@%s", repo, ref.Digest), nil
	} else if ref.Tag != "" {
		return fmt.Sprintf("%s:%s", repo, ref.Tag), nil
	} else {
		return fmt.Sprintf("%s:latest", repo), nil
	}
}

var _ ProtoMarshaler = ImageRef{}
var _ ProtoUnmarshaler = (*ImageRef)(nil)

func (ref *ImageRef) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ImageRef)
	if !ok {
		return DecodeError{msg, ref}
	}

	if err := ref.Platform.UnmarshalProto(p.Platform); err != nil {
		return fmt.Errorf("platform: %w", err)
	}

	switch repo := p.GetSource().(type) {
	case *proto.ImageRef_Repository:
		ref.Repository.Static = repo.Repository
	case *proto.ImageRef_Addr:
		ref.Repository.Addr = &ThunkAddr{}
		if err := ref.Repository.Addr.UnmarshalProto(repo.Addr); err != nil {
			return fmt.Errorf("repository addr: %w", err)
		}
	}

	ref.Tag = p.GetTag()
	ref.Digest = p.GetDigest()

	return nil
}

func (ref ImageRef) MarshalProto() (proto.Message, error) {
	pv := &proto.ImageRef{
		Platform: &proto.Platform{
			Os:   ref.Platform.OS,
			Arch: ref.Platform.Architecture,
		},
	}

	if ref.Tag != "" {
		pv.Tag = &ref.Tag
	}

	if ref.Digest != "" {
		pv.Digest = &ref.Digest
	}

	if ref.Repository.Static != "" {
		pv.Source = &proto.ImageRef_Repository{
			Repository: ref.Repository.Static,
		}
	} else if ref.Repository.Addr != nil {
		tp, err := ref.Repository.Addr.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("file: %w", err)
		}

		pv.Source = &proto.ImageRef_Addr{
			Addr: tp.(*proto.ThunkAddr),
		}
	}

	return pv, nil
}

// Platform configures an OCI image platform.
type Platform ocispecs.Platform

type bassPlatform struct {
	ocispecs.Platform

	// allow architecture to be omitted; default to runtime.GOOS
	Architecture string `json:"architecture,omitempty"`
}

func (platform Platform) String() string {
	return platforms.Format(ocispecs.Platform(platform))
}

func (platform *Platform) FromValue(val Value) error {
	var p ocispecs.Platform

	// default to current architecture
	p.Architecture = runtime.GOARCH

	if err := val.Decode(&p); err != nil {
		return err
	}

	*platform = Platform(p)

	return nil
}

func (platform *Platform) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.Platform)
	if !ok {
		return DecodeError{msg, platform}
	}

	platform.OS = p.Os
	platform.Architecture = p.Arch

	return nil
}

// LinuxPlatform is the minimum configuration to select a Linux runtime.
var LinuxPlatform = Platform{
	OS:           "linux",
	Architecture: runtime.GOARCH,
}

// CanSelect returns true if the given platform (from a runtime) matches.
func (platform Platform) CanSelect(given Platform) bool {
	return platforms.NewMatcher(ocispecs.Platform(platform)).Match(ocispecs.Platform(given))
}

type ThunkMountSource struct {
	ThunkPath *ThunkPath
	HostPath  *HostPath
	FSPath    *FSPath
	Cache     *CachePath
	Secret    *Secret
}

func (mount *ThunkMountSource) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ThunkMountSource)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, mount})
	}

	switch x := p.GetSource().(type) {
	case *proto.ThunkMountSource_Thunk:
		mount.ThunkPath = &ThunkPath{}
		return mount.ThunkPath.UnmarshalProto(x.Thunk)
	case *proto.ThunkMountSource_Host:
		mount.HostPath = &HostPath{}
		return mount.HostPath.UnmarshalProto(x.Host)
	case *proto.ThunkMountSource_Cache:
		mount.Cache = &CachePath{}
		return mount.Cache.UnmarshalProto(x.Cache)
	case *proto.ThunkMountSource_Logical:
		mount.FSPath = &FSPath{}
		return mount.FSPath.UnmarshalProto(x.Logical)
	case *proto.ThunkMountSource_Secret:
		mount.Secret = &Secret{}
		return mount.Secret.UnmarshalProto(x.Secret)
	default:
		return fmt.Errorf("unmarshal proto: unknown type: %T", x)
	}
}

func (src ThunkMountSource) MarshalProto() (proto.Message, error) {
	pv := &proto.ThunkMountSource{}

	if src.ThunkPath != nil {
		tp, err := src.ThunkPath.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Source = &proto.ThunkMountSource_Thunk{
			Thunk: tp.(*proto.ThunkPath),
		}
	} else if src.HostPath != nil {
		ppv, err := src.HostPath.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Source = &proto.ThunkMountSource_Host{
			Host: ppv.(*proto.HostPath),
		}
	} else if src.FSPath != nil {
		ppv, err := src.FSPath.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Source = &proto.ThunkMountSource_Logical{
			Logical: ppv.(*proto.LogicalPath),
		}
	} else if src.Cache != nil {
		p, err := src.Cache.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Source = &proto.ThunkMountSource_Cache{
			Cache: p.(*proto.CachePath),
		}
	} else if src.Secret != nil {
		ppv, err := src.Secret.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Source = &proto.ThunkMountSource_Secret{
			Secret: ppv.(*proto.Secret),
		}
	} else {
		return nil, fmt.Errorf("unexpected mount source type: %T", src.ToValue())
	}

	return pv, nil
}

var _ Decodable = &ThunkMountSource{}
var _ Encodable = ThunkMountSource{}

func (enum ThunkMountSource) ToValue() Value {
	if enum.FSPath != nil {
		return enum.FSPath
	} else if enum.HostPath != nil {
		return *enum.HostPath
	} else if enum.Cache != nil {
		return *enum.Cache
	} else if enum.Secret != nil {
		return *enum.Secret
	} else {
		return *enum.ThunkPath
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

	var fs *FSPath
	if err := val.Decode(&fs); err == nil {
		enum.FSPath = fs
		return nil
	}

	var tp ThunkPath
	if err := val.Decode(&tp); err == nil {
		enum.ThunkPath = &tp
		return nil
	}

	var cache CachePath
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
	Ref         *ImageRef
	Archive     *ImageArchive
	Thunk       *Thunk
	DockerBuild *ImageDockerBuild
}

func (img *ThunkImage) UnmarshalProto(msg proto.Message) error {
	protoImage, ok := msg.(*proto.ThunkImage)
	if !ok {
		return DecodeError{msg, img}
	}

	if protoImage.GetRef() != nil {
		i := protoImage.GetRef()

		// TODO: pre-0.10.0 backwards compatibility
		if i.GetFile() != nil {
			img.Archive = &ImageArchive{}

			if err := img.Archive.File.UnmarshalProto(i.GetFile()); err != nil {
				return err
			}

			if err := img.Archive.Platform.UnmarshalProto(i.GetPlatform()); err != nil {
				return err
			}

			img.Archive.Tag = i.GetTag()
		} else {
			img.Ref = &ImageRef{}

			if i.GetRepository() != "" {
				img.Ref.Repository.Static = i.GetRepository()
			}

			if i.GetAddr() != nil {
				img.Ref.Repository.Addr = &ThunkAddr{}
				if err := img.Ref.Repository.Addr.UnmarshalProto(i.GetAddr()); err != nil {
					return err
				}
			}

			if err := img.Ref.Platform.UnmarshalProto(i.GetPlatform()); err != nil {
				return err
			}

			img.Ref.Tag = i.GetTag()
			img.Ref.Digest = i.GetDigest()
		}
	} else if protoImage.GetThunk() != nil {
		img.Thunk = &Thunk{}
		if err := img.Thunk.UnmarshalProto(protoImage.GetThunk()); err != nil {
			return err
		}
	} else if protoImage.GetArchive() != nil {
		img.Archive = &ImageArchive{}
		if err := img.Archive.UnmarshalProto(protoImage.GetArchive()); err != nil {
			return err
		}
	} else if protoImage.GetDockerBuild() != nil {
		img.DockerBuild = &ImageDockerBuild{}
		if err := img.DockerBuild.UnmarshalProto(protoImage.GetDockerBuild()); err != nil {
			return err
		}
	}

	return nil
}

func (img ThunkImage) MarshalProto() (proto.Message, error) {
	ti := &proto.ThunkImage{}

	if img.Ref != nil {
		ri, err := img.Ref.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("ref: %w", err)
		}

		ti.Image = &proto.ThunkImage_Ref{
			Ref: ri.(*proto.ImageRef),
		}
	} else if img.Thunk != nil {
		tv, err := img.Thunk.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("parent: %w", err)
		}

		ti.Image = &proto.ThunkImage_Thunk{
			Thunk: tv.(*proto.Thunk),
		}
	} else if img.Archive != nil {
		p, err := img.Archive.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("parent: %w", err)
		}

		ti.Image = &proto.ThunkImage_Archive{
			Archive: p.(*proto.ImageArchive),
		}
	} else if img.DockerBuild != nil {
		p, err := img.DockerBuild.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("parent: %w", err)
		}

		ti.Image = &proto.ThunkImage_DockerBuild{
			DockerBuild: p.(*proto.ImageDockerBuild),
		}
	} else {
		return nil, fmt.Errorf("unexpected image type: %T", img.ToValue())
	}

	return ti, nil
}

func (img ThunkImage) Platform() *Platform {
	if img.Ref != nil {
		return &img.Ref.Platform
	} else if img.Thunk != nil {
		return img.Thunk.Platform()
	} else if img.Archive != nil {
		return &img.Archive.Platform
	} else if img.DockerBuild != nil {
		return &img.DockerBuild.Platform
	} else {
		return nil
	}
}

var _ Decodable = &ThunkImage{}
var _ Encodable = ThunkImage{}

func (image ThunkImage) ToValue() Value {
	if image.Ref != nil {
		val, _ := ValueOf(*image.Ref)
		return val
	} else if image.Thunk != nil {
		return *image.Thunk
	} else if image.Archive != nil {
		val, _ := ValueOf(*image.Archive)
		return val
	} else if image.DockerBuild != nil {
		val, _ := ValueOf(*image.DockerBuild)
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

	var archive ImageArchive
	if err := val.Decode(&archive); err == nil {
		image.Archive = &archive
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", val, err))
	}

	var dockerBuild ImageDockerBuild
	if err := val.Decode(&dockerBuild); err == nil {
		image.DockerBuild = &dockerBuild
		return nil
	} else {
		errs = multierror.Append(errs, fmt.Errorf("%T: %w", val, err))
	}

	return fmt.Errorf("image enum: %w", errs)
}

type ThunkDir struct {
	Dir      *DirPath
	ThunkDir *ThunkPath
	HostDir  *HostPath
}

func (dir *ThunkDir) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ThunkDir)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, dir})
	}

	switch x := p.GetDir().(type) {
	case *proto.ThunkDir_Local:
		dir.Dir = &DirPath{}
		return dir.Dir.UnmarshalProto(x.Local)
	case *proto.ThunkDir_Thunk:
		dir.ThunkDir = &ThunkPath{}
		return dir.ThunkDir.UnmarshalProto(x.Thunk)
	case *proto.ThunkDir_Host:
		dir.HostDir = &HostPath{}
		return dir.HostDir.UnmarshalProto(x.Host)
	default:
		return fmt.Errorf("unmarshal proto: unknown type: %T", x)
	}
}

func (dir ThunkDir) MarshalProto() (proto.Message, error) {
	pv := &proto.ThunkDir{}

	if dir.Dir != nil {
		dv, err := dir.Dir.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Dir = &proto.ThunkDir_Local{
			Local: dv.(*proto.DirPath),
		}
	} else if dir.ThunkDir != nil {
		cv, err := dir.ThunkDir.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Dir = &proto.ThunkDir_Thunk{
			Thunk: cv.(*proto.ThunkPath),
		}
	} else if dir.HostDir != nil {
		cv, err := dir.HostDir.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Dir = &proto.ThunkDir_Host{
			Host: cv.(*proto.HostPath),
		}
	} else {
		return nil, fmt.Errorf("unexpected dir type: %T", dir.ToValue())
	}

	return pv, nil
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

type ImageRepository struct {
	Static string
	Addr   *ThunkAddr
}

var _ Decodable = &ImageRepository{}
var _ Encodable = ImageRepository{}

// ToValue returns the value present.
func (path ImageRepository) ToValue() Value {
	if path.Static != "" {
		return String(path.Static)
	} else {
		return *path.Addr
	}
}

// FromValue decodes val into a FilePath or a DirPath, setting whichever worked
// as the internal value.
func (repo *ImageRepository) FromValue(val Value) error {
	var str string
	if err := val.Decode(&str); err == nil {
		repo.Static = str
		return nil
	}

	var addr ThunkAddr
	if err := val.Decode(&addr); err == nil {
		repo.Addr = &addr
		return nil
	}

	return DecodeError{
		Source:      val,
		Destination: repo,
	}
}

// ImageArchive specifies an OCI image tarball.
type ImageArchive struct {
	// An OCI image archive tarball to load.
	File ThunkPath `json:"file"`

	// The platform to target; influences runtime selection.
	Platform Platform `json:"platform"`

	// The tag to use from the archive.
	Tag string `json:"tag,omitempty"`
}

var _ ProtoMarshaler = ImageArchive{}
var _ ProtoUnmarshaler = (*ImageArchive)(nil)

func (ref *ImageArchive) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ImageArchive)
	if !ok {
		return DecodeError{msg, ref}
	}

	if err := ref.File.UnmarshalProto(p.GetFile()); err != nil {
		return fmt.Errorf("file: %w", err)
	}

	if err := ref.Platform.UnmarshalProto(p.GetPlatform()); err != nil {
		return fmt.Errorf("platform: %w", err)
	}

	ref.Tag = p.GetTag()

	return nil
}

func (ref ImageArchive) MarshalProto() (proto.Message, error) {
	pv := &proto.ImageArchive{
		Platform: &proto.Platform{
			Os:   ref.Platform.OS,
			Arch: ref.Platform.Architecture,
		},
	}

	if ref.Tag != "" {
		pv.Tag = &ref.Tag
	}

	tp, err := ref.File.MarshalProto()
	if err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}

	pv.File = tp.(*proto.ThunkPath)

	return pv, nil
}

// ImageDockerBuild specifies an OCI image tarball.
type ImageDockerBuild struct {
	// The platform to target; influences runtime selection.
	Platform Platform `json:"platform"`

	// An OCI image archive tarball to load.
	Context ImageBuildInput `json:"docker_build"`

	// Path to a Dockerfile to use within the context.
	Dockerfile FilePath `json:"dockerfile,omitempty"`

	// Target witin the Dockerfile to build.
	Target string `json:"target,omitempty"`

	// Arbitrary key-value args to pass to the build.
	Args *Scope `json:"args,omitempty"`
}

var _ ProtoMarshaler = ImageDockerBuild{}
var _ ProtoUnmarshaler = (*ImageDockerBuild)(nil)

func (ref *ImageDockerBuild) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ImageDockerBuild)
	if !ok {
		return DecodeError{msg, ref}
	}

	if err := ref.Platform.UnmarshalProto(p.GetPlatform()); err != nil {
		return fmt.Errorf("platform: %w", err)
	}

	if err := ref.Context.UnmarshalProto(p.GetContext()); err != nil {
		return fmt.Errorf("platform: %w", err)
	}

	if p.GetDockerfile() != "" {
		ref.Dockerfile = NewFilePath(p.GetDockerfile())
	}

	ref.Target = p.GetTarget()

	ref.Args = NewEmptyScope()
	for _, arg := range p.GetArgs() {
		ref.Args.Set(Symbol(arg.GetName()), String(arg.GetValue()))
	}

	return nil
}

func (ref ImageDockerBuild) MarshalProto() (proto.Message, error) {
	pv := &proto.ImageDockerBuild{
		Platform: &proto.Platform{
			Os:   ref.Platform.OS,
			Arch: ref.Platform.Architecture,
		},
	}

	i, err := ref.Context.MarshalProto()
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	pv.Context = i.(*proto.ImageBuildInput)

	return pv, nil
}

type ImageBuildInput struct {
	Thunk *ThunkPath
	Host  *HostPath
	FS    *FSPath
}

func (ref *ImageBuildInput) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.ImageBuildInput)
	if !ok {
		return DecodeError{msg, ref}
	}

	switch input := p.GetInput().(type) {
	case *proto.ImageBuildInput_Thunk:
		ref.Thunk = &ThunkPath{}
		if err := ref.Thunk.UnmarshalProto(input.Thunk); err != nil {
			return fmt.Errorf("repository addr: %w", err)
		}
	case *proto.ImageBuildInput_Host:
		ref.Host = &HostPath{}
		if err := ref.Host.UnmarshalProto(input.Host); err != nil {
			return fmt.Errorf("repository addr: %w", err)
		}
	case *proto.ImageBuildInput_Logical:
		ref.FS = &FSPath{}
		if err := ref.FS.UnmarshalProto(input.Logical); err != nil {
			return fmt.Errorf("repository addr: %w", err)
		}
	}

	return nil
}

func (ref ImageBuildInput) MarshalProto() (proto.Message, error) {
	pv := &proto.ImageBuildInput{}

	if ref.Thunk != nil {
		path, err := ref.Thunk.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		pv.Input = &proto.ImageBuildInput_Thunk{
			Thunk: path.(*proto.ThunkPath),
		}
	} else if ref.Host != nil {
		path, err := ref.Host.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		pv.Input = &proto.ImageBuildInput_Host{
			Host: path.(*proto.HostPath),
		}
	} else if ref.FS != nil {
		path, err := ref.FS.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		pv.Input = &proto.ImageBuildInput_Logical{
			Logical: path.(*proto.LogicalPath),
		}
	}

	return pv, nil
}

var _ Decodable = &ImageBuildInput{}
var _ Encodable = ImageBuildInput{}

func (enum ImageBuildInput) ToValue() Value {
	if enum.FS != nil {
		return enum.FS
	} else if enum.Host != nil {
		return *enum.Host
	} else if enum.Thunk != nil {
		return *enum.Thunk
	} else {
		panic("empty ImageBuildInput")
	}
}

func (enum *ImageBuildInput) UnmarshalJSON(payload []byte) error {
	return UnmarshalJSON(payload, enum)
}

func (enum ImageBuildInput) MarshalJSON() ([]byte, error) {
	return MarshalJSON(enum.ToValue())
}

func (enum *ImageBuildInput) FromValue(val Value) error {
	var host HostPath
	if err := val.Decode(&host); err == nil {
		enum.Host = &host
		return nil
	}

	var fs *FSPath
	if err := val.Decode(&fs); err == nil {
		enum.FS = fs
		return nil
	}

	var tp ThunkPath
	if err := val.Decode(&tp); err == nil {
		enum.Thunk = &tp
		return nil
	}

	return DecodeError{
		Source:      val,
		Destination: enum,
	}
}
