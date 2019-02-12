package solver

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

type ref struct {
	ssa.Value
}

func (v *ref) Type() types.Type {
	return v.Value.Type().(*types.Pointer).Elem()
}
