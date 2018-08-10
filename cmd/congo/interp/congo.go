package interp

import (
	"go/types"
)

func convertSymbolicValuesToInterpRepr(v interface{}, t types.Type) value {
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
		return convertSymbolicValuesToInterpRepr(v, t.Underlying())
	}
	return nil
}
