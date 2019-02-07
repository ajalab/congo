package interp

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

const congoSymbolPackagePath = "github.com/ajalab/congo/symbol"

type SymbolicValue struct {
	Value interface{}
	Type  types.Type
}

func value2InterpValue(v interface{}, t types.Type) value {
	switch t := t.(type) {
	case *types.Basic:
		return v
	case *types.Struct:
		vs := v.([]interface{})
		values := make(structure, len(vs))
		for i, v := range vs {
			values[i] = v.(value)
		}
		return values
	case *types.Named:
		return value2InterpValue(v, t.Underlying())
	case *types.Pointer:
		a := v.(*interface{})
		if a == nil {
			return (*value)(nil)
		}
		val := value2InterpValue(*a, t.Elem())
		return &val
	}
	return nil
}

// CongoInterpResult is the type that contains interp.Interp result
type CongoInterpResult struct {
	ExitCode    int
	Instrs      []ssa.Instruction
	ReturnValue interface{}
}
