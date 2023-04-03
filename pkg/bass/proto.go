package bass

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/vito/bass/pkg/proto"
)

type ProtoMarshaler interface {
	MarshalProto() (proto.Message, error)
}

type ProtoUnmarshaler interface {
	UnmarshalProto(proto.Message) error
}

func MarshalProto(val Value) (*proto.Value, error) {
	dm, ok := val.(ProtoMarshaler)
	if !ok {
		return nil, fmt.Errorf("%T is not a ProtoMarshaler", val)
	}

	d, err := dm.MarshalProto()
	if err != nil {
		return nil, err
	}

	return proto.NewValue(d)
}

func FromProto(val *proto.Value) (Value, error) {
	switch x := val.GetValue().(type) {
	case *proto.Value_Null:
		return Null{}, nil
	case *proto.Value_Bool:
		return Bool(x.Bool.Value), nil
	case *proto.Value_Int:
		return Int(x.Int.Value), nil
	case *proto.Value_String_:
		return String(x.String_.Value), nil
	case *proto.Value_Secret:
		return NewSecret(x.Secret.Name, nil), nil
	case *proto.Value_Array:
		var vals []Value
		for i, v := range x.Array.Values {
			val, err := FromProto(v)
			if err != nil {
				return nil, fmt.Errorf("unmarshal array[%d]: %w", i, err)
			}

			vals = append(vals, val)
		}

		return NewList(vals...), nil
	case *proto.Value_Object:
		scope := NewEmptyScope()
		for i, bnd := range x.Object.Bindings {
			val, err := FromProto(bnd.Value)
			if err != nil {
				return nil, fmt.Errorf("unmarshal array[%d]: %w", i, err)
			}

			scope.Set(Symbol(bnd.Symbol), val)
		}

		return scope, nil
	case *proto.Value_FilePath:
		return FilePath{Path: x.FilePath.Path}, nil
	case *proto.Value_DirPath:
		return DirPath{Path: x.DirPath.Path}, nil
	case *proto.Value_HostPath:
		return HostPath{
			ContextDir: x.HostPath.Context,
			Path:       fod(x.HostPath.Path),
		}, nil
	case *proto.Value_CachePath:
		return CachePath{
			ID:   x.CachePath.Id,
			Path: fod(x.CachePath.Path),
		}, nil
	case *proto.Value_LogicalPath:
		fsp := &FSPath{}
		if err := fsp.UnmarshalProto(x.LogicalPath); err != nil {
			return nil, err
		}

		return fsp, nil
	case *proto.Value_Thunk:
		var thunk Thunk
		if err := thunk.UnmarshalProto(x.Thunk); err != nil {
			return nil, err
		}

		return thunk, nil
	case *proto.Value_ThunkPath:
		var tp ThunkPath
		if err := tp.UnmarshalProto(x.ThunkPath); err != nil {
			return nil, err
		}

		return tp, nil
	case *proto.Value_CommandPath:
		return CommandPath{x.CommandPath.Name}, nil
	case *proto.Value_ThunkAddr:
		var ta ThunkAddr
		if err := ta.UnmarshalProto(x.ThunkAddr); err != nil {
			return nil, err
		}

		return ta, nil
	default:
		return nil, fmt.Errorf("unexpected type %T", x)
	}
}

func fod(p *proto.FilesystemPath) FileOrDirPath {
	if p.GetDir() != nil {
		return FileOrDirPath{
			Dir: &DirPath{Path: p.GetDir().GetPath()},
		}
	} else {
		return FileOrDirPath{
			File: &FilePath{Path: p.GetFile().GetPath()},
		}
	}
}

func (value Null) MarshalProto() (proto.Message, error) {
	return &proto.Null{}, nil
}

func (value Bool) MarshalProto() (proto.Message, error) {
	return &proto.Bool{Value: bool(value)}, nil
}

func (value Int) MarshalProto() (proto.Message, error) {
	return &proto.Int{Value: int64(value)}, nil
}

func (value String) MarshalProto() (proto.Message, error) {
	return &proto.String{Value: string(value)}, nil
}

func (value Secret) MarshalProto() (proto.Message, error) {
	return &proto.Secret{
		Name: value.Name,
	}, nil
}

func (Empty) MarshalProto() (proto.Message, error) {
	return &proto.Array{}, nil
}

func (value Pair) MarshalProto() (proto.Message, error) {
	vs, err := ToSlice(value)
	if err != nil {
		return nil, err
	}

	pvs := make([]*proto.Value, len(vs))
	for i, v := range vs {
		pvs[i], err = MarshalProto(v)
		if err != nil {
			return nil, fmt.Errorf("%d: %w", i, err)
		}
	}

	return &proto.Array{Values: pvs}, nil
}

func (value *Scope) MarshalProto() (proto.Message, error) {
	var bindings []*proto.Binding
	err := value.Each(func(sym Symbol, val Value) error {
		v, err := MarshalProto(val)
		if err != nil {
			return fmt.Errorf("%s: %w", sym, err)
		}

		bindings = append(bindings, &proto.Binding{
			Symbol: string(sym),
			Value:  v,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &proto.Object{
		Bindings: bindings,
	}, nil
}

func (value FilePath) MarshalProto() (proto.Message, error) {
	return &proto.FilePath{
		Path: value.Path,
	}, nil
}

func (value DirPath) MarshalProto() (proto.Message, error) {
	return &proto.DirPath{
		Path: value.Path,
	}, nil
}

func (fod FileOrDirPath) MarshalProto() (proto.Message, error) {
	pv := &proto.FilesystemPath{}

	if fod.File != nil {
		f, err := fod.File.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Path = &proto.FilesystemPath_File{File: f.(*proto.FilePath)}
	} else if fod.Dir != nil {
		d, err := fod.Dir.MarshalProto()
		if err != nil {
			return nil, err
		}

		pv.Path = &proto.FilesystemPath_Dir{Dir: d.(*proto.DirPath)}
	} else {
		return nil, fmt.Errorf("unknown type %T", fod.ToValue())
	}

	return pv, nil
}

func (value HostPath) MarshalProto() (proto.Message, error) {
	pv := &proto.HostPath{
		Context: value.ContextDir,
	}

	pathp, err := value.Path.MarshalProto()
	if err != nil {
		return nil, err
	}

	pv.Path = pathp.(*proto.FilesystemPath)

	return pv, nil
}

func (value *FSPath) MarshalProto() (proto.Message, error) {
	fsp := value.Path.FilesystemPath()

	lp := &proto.LogicalPath{}

	if fsp.IsDir() {
		dir := &proto.LogicalPath_Dir{
			Name: value.Name(),
		}

		ents, err := fs.ReadDir(value.FS, path.Clean(fsp.Slash()))
		if err != nil {
			return nil, fmt.Errorf("marshal fs dir: %w", err)
		}

		for _, ent := range ents {
			var sub Path
			if ent.IsDir() {
				sub = DirPath{
					Path: ent.Name(),
				}
			} else {
				sub = FilePath{
					Path: ent.Name(),
				}
			}

			subfs, err := value.Extend(sub)
			if err != nil {
				return nil, err
			}

			p, err := subfs.(*FSPath).MarshalProto()
			if err != nil {
				return nil, err
			}

			dir.Entries = append(dir.Entries, p.(*proto.LogicalPath))
		}

		lp.Path = &proto.LogicalPath_Dir_{
			Dir: dir,
		}
	} else {
		content, err := fs.ReadFile(value.FS, path.Clean(fsp.Slash()))
		if err != nil {
			return nil, fmt.Errorf("marshal fs %s: %w", fsp, err)
		}

		lp.Path = &proto.LogicalPath_File_{
			File: &proto.LogicalPath_File{
				Name:    value.Name(),
				Content: content,
			},
		}
	}

	return lp, nil
}

func (value CachePath) MarshalProto() (proto.Message, error) {
	pv := &proto.CachePath{
		Id: value.ID,
	}

	pathp, err := value.Path.MarshalProto()
	if err != nil {
		return nil, err
	}

	pv.Path = pathp.(*proto.FilesystemPath)

	return pv, nil
}

func (value Thunk) MarshalProto() (proto.Message, error) {
	thunk := &proto.Thunk{
		Insecure: value.Insecure,
	}

	if value.Image != nil {
		ti, err := value.Image.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("image: %w", err)
		}

		thunk.Image = ti.(*proto.ThunkImage)
	}

	for i, v := range value.Args {
		pv, err := MarshalProto(v)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}

		thunk.Args = append(thunk.Args, pv)
	}

	for i, v := range value.Stdin {
		pv, err := MarshalProto(v)
		if err != nil {
			return nil, fmt.Errorf("stdin %d: %w", i, err)
		}

		thunk.Stdin = append(thunk.Stdin, pv)
	}

	if value.Env != nil {
		err := value.Env.Each(func(sym Symbol, val Value) error {
			pv, err := MarshalProto(val)
			if err != nil {
				return fmt.Errorf("%s: %w", sym, err)
			}

			thunk.Env = append(thunk.Env, &proto.Binding{
				Symbol: string(sym),
				Value:  pv,
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("env: %w", err)
		}
	}

	if value.Dir != nil {
		di, err := value.Dir.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("dir: %w", err)
		}

		thunk.Dir = di.(*proto.ThunkDir)
	}

	for _, m := range value.Mounts {
		pm, err := m.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("dir: %w", err)
		}

		thunk.Mounts = append(thunk.Mounts, pm.(*proto.ThunkMount))
	}

	if value.Labels != nil {
		err := value.Labels.Each(func(sym Symbol, val Value) error {
			lv, err := MarshalProto(val)
			if err != nil {
				return fmt.Errorf("%s: %w", sym, err)
			}

			thunk.Labels = append(thunk.Labels, &proto.Binding{
				Symbol: string(sym),
				Value:  lv,
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("labels: %w", err)
		}
	}

	for _, port := range value.Ports {
		thunk.Ports = append(thunk.Ports, &proto.ThunkPort{
			Name: port.Name,
			Port: int32(port.Port),
		})
	}

	if value.TLS != nil {
		cert, err := value.TLS.Cert.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("marshal cert: %w", err)
		}

		key, err := value.TLS.Key.MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("marshal cert: %w", err)
		}

		thunk.Tls = &proto.ThunkTLS{
			Cert: cert.(*proto.FilePath),
			Key:  key.(*proto.FilePath),
		}
	}

	return thunk, nil
}

func (value ThunkPath) MarshalProto() (proto.Message, error) {
	t, err := value.Thunk.MarshalProto()
	if err != nil {
		return nil, fmt.Errorf("thunk: %w", err)
	}

	pv := &proto.ThunkPath{
		Thunk: t.(*proto.Thunk),
	}

	pathp, err := value.Path.MarshalProto()
	if err != nil {
		return nil, err
	}

	pv.Path = pathp.(*proto.FilesystemPath)

	return pv, nil
}

func (value CommandPath) MarshalProto() (proto.Message, error) {
	return &proto.CommandPath{
		Name: value.Command,
	}, nil
}
