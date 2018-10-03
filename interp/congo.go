package interp

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

const packageCongoSymbolPath = "github.com/ajalab/congo/symbol"
const packageRunnerPath = "congomain"

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
			return nil
		}
		return &*a
	}
	return nil
}

// CongoInterpResult is the type that contains interp.Interp result
type CongoInterpResult struct {
	ExitCode    int
	Trace       []*ssa.BasicBlock
	ReturnValue interface{}
}
