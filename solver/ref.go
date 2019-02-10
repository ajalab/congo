package solver

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

type Ref struct {
	ssa.Value
}

func (v *Ref) Type() types.Type {
	return v.Value.Type().(*types.Pointer).Elem()
}
