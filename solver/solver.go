package solver

import "golang.org/x/tools/go/ssa"

// Branch represents a branching instruction that appeared in a running trace with some additional information.
// This includes instructions that may cause a panic (e.g., pointer dereference) as well as ordinary branching by *ssa.If.
type Branch interface {
	// Instr returns ssa.Instruction value for the branch.
	Instr() ssa.Instruction
}

// BranchIf contains a branching instruction (*ssa.If) and
// the direction taken in the concolic execution.
type BranchIf struct {
	instr     *ssa.If
	Direction bool
}

func (b *BranchIf) Instr() ssa.Instruction {
	return b.instr
}

func (b *BranchIf) Succs() []*ssa.BasicBlock {
	return b.instr.Block().Succs
}

// PanicNilPointerDeref represents a (panic) branching caused by nil pointer dereference.
type PanicNilPointerDeref struct {
	instr ssa.Instruction
	x     ssa.Value
}

func (p *PanicNilPointerDeref) Instr() ssa.Instruction {
	return p.instr
}

// UnsatError is an error describing that Z3 constraints were unsatisfied.
type UnsatError struct{}

func (ue UnsatError) Error() string {
	return "unsat"
}
