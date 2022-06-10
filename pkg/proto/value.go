package proto

import "fmt"

func NewValue(msg Message) (*Value, error) {
	var val Value

	switch x := msg.(type) {
	case *Bool:
		val.Value = &Value_Bool{x}
	case *Int:
		val.Value = &Value_Int{x}
	case *String:
		val.Value = &Value_String_{x}
	case *Secret:
		val.Value = &Value_Secret{x}
	case *Array:
		val.Value = &Value_Array{x}
	case *Object:
		val.Value = &Value_Object{x}
	case *FilePath:
		val.Value = &Value_FilePath{x}
	case *DirPath:
		val.Value = &Value_DirPath{x}
	case *HostPath:
		val.Value = &Value_HostPath{x}
	case *FSPath:
		val.Value = &Value_FsPath{x}
	case *Thunk:
		val.Value = &Value_Thunk{x}
	case *ThunkPath:
		val.Value = &Value_ThunkPath{x}
	case *CommandPath:
		val.Value = &Value_CommandPath{x}
	case *Null:
		val.Value = &Value_Null{x}
	default:
		return nil, fmt.Errorf("cannot convert to *Value: %T", x)
	}

	return &val, nil
}
