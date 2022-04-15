package proto

import "fmt"

func NewValue(msg Message) (*Value, error) {
	var val Value

	switch x := msg.(type) {
	case *Bool:
		val.Value = &Value_BoolValue{x}
	case *Int:
		val.Value = &Value_IntValue{x}
	case *String:
		val.Value = &Value_StringValue{x}
	case *Secret:
		val.Value = &Value_SecretValue{x}
	case *Empty:
		val.Value = &Value_EmptyValue{x}
	case *Pair:
		val.Value = &Value_PairValue{x}
	case *Scope:
		val.Value = &Value_ScopeValue{x}
	case *FilePath:
		val.Value = &Value_FilePathValue{x}
	case *DirPath:
		val.Value = &Value_DirPathValue{x}
	case *HostPath:
		val.Value = &Value_HostPathValue{x}
	case *FSPath:
		val.Value = &Value_FsPathValue{x}
	case *Thunk:
		val.Value = &Value_ThunkValue{x}
	case *ThunkPath:
		val.Value = &Value_ThunkPathValue{x}
	case *CommandPath:
		val.Value = &Value_CommandPathValue{x}
	case *Null:
		val.Value = &Value_NullValue{x}
	default:
		return nil, fmt.Errorf("cannot convert to *Value: %T", x)
	}

	return &val, nil
}
