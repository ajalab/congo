package solver

import "golang.org/x/tools/go/ssa"

// Branch represents a branch appeared in a running trace.
// This includes instructions that may cause a panic (e.g., pointer dereference) as well as ordinary branching by *ssa.If.
type Branch interface {
	// Instr returns ssa.Instruction value for the branch.
	Instr() ssa.Instruction

	// To returns ssa.BasicBlock that the branch took.
	To() *ssa.BasicBlock

	// Other returns ssa.BasicBlock that the branch did not take.
	Other() *ssa.BasicBlock
}

// BranchIf contains a branching instruction (*ssa.If) and
// the direction taken in the concolic execution.
type BranchIf struct {
	instr     *ssa.If
	direction bool
}

// Instr returns ssa.Instruction value for the branch.
func (b *BranchIf) Instr() ssa.Instruction {
	return b.instr
}

// To returns ssa.BasicBlock that the branch took.
func (b *BranchIf) To() *ssa.BasicBlock {
	succs := b.instr.Block().Succs
	if b.direction {
		return succs[0]
	}
	return succs[1]
}

// Other returns ssa.BasicBlock that the branch did not take.
func (b *BranchIf) Other() *ssa.BasicBlock {
	succs := b.instr.Block().Succs
	if b.direction {
		return succs[1]
	}
	return succs[0]
}

// BranchDeref represents a branching (success or panic) caused by
// dereference (*ssa.UnOp or *ssa.FieldAddr).
type BranchDeref struct {
	instr   ssa.Instruction
	success bool
	x       ssa.Value
}

// Instr returns ssa.Instruction value for the branch.
func (b *BranchDeref) Instr() ssa.Instruction {
	return b.instr
}

// To returns ssa.BasicBlock that the branch took.
func (b *BranchDeref) To() *ssa.BasicBlock {
	if b.success {
		return b.instr.Block()
	}
	return nil
}

// Other returns ssa.BasicBlock that the branch did not take.
func (b *BranchDeref) Other() *ssa.BasicBlock {
	if b.success {
		return nil
	}
	return b.instr.Block()
}
