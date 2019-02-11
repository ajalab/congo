package solver

import "go/types"

// Solution represents a solution for a symbol.
type Solution interface {
	Type() types.Type
	Concretize(func(types.Type) interface{}) interface{}
}

// Definite represents a solution.
// If ty is *types.Pointer, then the value is an instance of Solution
// which represents the referenced value.
type Definite struct {
	ty    types.Type
	value interface{}
}

// Type returns type.
func (s Definite) Type() types.Type {
	return s.ty
}

// Concretize returns the value.
func (s Definite) Concretize(f func(types.Type) interface{}) interface{} {
	if _, ok := s.ty.(*types.Pointer); ok {
		if s.value == nil {
			return (*interface{})(nil)
		}
		if subs, ok := s.value.(Solution); ok {
			// Heap allocation
			v := subs.Concretize(f)
			return &v
		}
		//TODO(ajalab): remove panic
		panic("unreachable")
	}
	return s.value
}

// Indefinite represents an indefinite solution.
type Indefinite struct {
	ty types.Type
}

// Type returns type.
func (s Indefinite) Type() types.Type {
	return s.ty
}

// Concretize returns the value.
func (s Indefinite) Concretize(f func(types.Type) interface{}) interface{} {
	return f(s.ty)
}

// NewIndefinite returns a new indefinite solution.
func NewIndefinite(ty types.Type) Indefinite {
	return Indefinite{ty: ty}
}
