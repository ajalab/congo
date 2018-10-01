package congo

import (
	"fmt"
	"go/types"
	"unsafe"

	"golang.org/x/tools/go/ssa"
)

func zero(ty types.Type) interface{} {
	switch t := ty.(type) {
	case *types.Basic:
		if t.Kind() == types.UntypedNil {
			panic("untyped nil has no zero value")
		}
		switch t.Kind() {
		case types.Bool:
			return false
		case types.Int:
			return int(0)
		case types.Int8:
			return int8(0)
		case types.Int16:
			return int16(0)
		case types.Int32:
			return int32(0)
		case types.Int64:
			return int64(0)
		case types.Uint:
			return uint(0)
		case types.Uint8:
			return uint8(0)
		case types.Uint16:
			return uint16(0)
		case types.Uint32:
			return uint32(0)
		case types.Uint64:
			return uint64(0)
		case types.Uintptr:
			return uintptr(0)
		case types.Float32:
			return float32(0)
		case types.Float64:
			return float64(0)
		case types.Complex64:
			return complex64(0)
		case types.Complex128:
			return complex128(0)
		case types.String:
			return ""
		case types.UnsafePointer:
			return unsafe.Pointer(nil)
		default:
			panic(fmt.Sprint("zero for unexpected type:", t))
		}
	case *types.Pointer:
		return (*interface{})(nil)
	case *types.Array:
		a := make([]interface{}, t.Len())
		for i := range a {
			a[i] = zero(t.Elem())
		}
		return a
	case *types.Named:
		return zero(t.Underlying())
	case *types.Interface:
		panic("unimplemented")
	case *types.Slice:
		return []interface{}(nil)
	case *types.Struct:
		s := make([]interface{}, t.NumFields())
		for i := range s {
			s[i] = zero(t.Field(i).Type())
		}
		return s
	case *types.Tuple:
		if t.Len() == 1 {
			return zero(t.At(0).Type())
		}
		s := make([]interface{}, t.Len())
		for i := range s {
			s[i] = zero(t.At(i).Type())
		}
		return s
	case *types.Chan:
		return chan interface{}(nil)
	case *types.Map:
		return map[interface{}][]interface{}(nil)
	case *types.Signature:
		return (*ssa.Function)(nil)
	}

	panic(fmt.Sprint("zero: unexpected ", ty))
}
