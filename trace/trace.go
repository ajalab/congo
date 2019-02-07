package trace

import "golang.org/x/tools/go/ssa"

// Trace is a sequence of instructions executed by an interpreter.
type Trace struct {
	instrs     []ssa.Instruction
	blocks     []*ssa.BasicBlock
	branches   []Branch
	isComplete bool
}

// NewTrace creates a new Trace.
func NewTrace(instrs []ssa.Instruction, blocks []*ssa.BasicBlock, isComplete bool) *Trace {
	var branches []Branch
	for i, b := range blocks {
		lastInstr := b.Instrs[len(b.Instrs)-1]
		if ifInstr, ok := lastInstr.(*ssa.If); ok {
			thenBlock := b.Succs[0]
			nextBlock := blocks[i+1]
			branches = append(branches, &If{
				instr:     ifInstr,
				Direction: thenBlock == nextBlock,
			})
		}
	}
	if !isComplete {
		causeInstr := instrs[len(instrs)-1]
		switch instr := causeInstr.(type) {
		case *ssa.UnOp:
			branches = append(branches, &PanicNilPointerDeref{
				instr: instr,
				x:     instr.X,
			})
		case *ssa.FieldAddr:
			branches = append(branches, &PanicNilPointerDeref{
				instr: instr,
				x:     instr.X,
			})
		}
	}

	return &Trace{
		instrs:     instrs,
		blocks:     blocks,
		branches:   branches,
		isComplete: isComplete,
	}
}

// Instrs returns a sequence of recorded instructions.
func (t *Trace) Instrs() []ssa.Instruction {
	return t.instrs
}

// Blocks returns a sequence of recorded blocks.
func (t *Trace) Blocks() []*ssa.BasicBlock {
	return t.blocks
}

// Branches returns a sequence of branches.
func (t *Trace) Branches() []Branch {
	return t.branches
}

// IsComplete returns true if the trace is generated from
// a successful run.
func (t *Trace) IsComplete() bool {
	return t.isComplete
}

// NumBranches returns the number of branches.
func (t *Trace) NumBranches() int {
	return len(t.branches)
}
