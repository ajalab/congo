package trace

import "golang.org/x/tools/go/ssa"

// Branch represents a branching instruction that appeared in a running trace with some additional information.
// This includes instructions that may cause a panic (e.g., pointer dereference) as well as ordinary branching by *ssa.If.
type Branch interface {
	// Instr returns ssa.Instruction value for the branch.
	Instr() ssa.Instruction
}

// If contains a branching instruction (*ssa.If) and
// the direction taken in the concolic execution.
type If struct {
	instr     *ssa.If
	Direction bool
}

// Instr returns ssa.Instruction value for the branch.
func (b *If) Instr() ssa.Instruction {
	return b.instr
}

// Cond returns ssa.Value that corresponds to the condition in the if statement
func (b *If) Cond() ssa.Value {
	return b.instr.Cond
}

// Succs returns the succeeding blocks.
func (b *If) Succs() []*ssa.BasicBlock {
	return b.instr.Block().Succs
}

// PanicNilPointerDeref represents a (panic) branching caused by nil pointer dereference.
type PanicNilPointerDeref struct {
	instr ssa.Instruction
	x     ssa.Value
}

// Instr returns ssa.Instruction value for the branch.
func (p *PanicNilPointerDeref) Instr() ssa.Instruction {
	return p.instr
}

// X returns ssa.Value that was dereferenced.
func (p *PanicNilPointerDeref) X() ssa.Value {
	return p.x
}
