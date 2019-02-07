package trace

import "golang.org/x/tools/go/ssa"

// Trace is a sequence of instructions executed by an interpreter.
type Trace struct {
	instrs     []ssa.Instruction
	isComplete bool
}

// NewTrace creates a new Trace.
func NewTrace(instrs []ssa.Instruction, isComplete bool) *Trace {
	return &Trace{
		instrs:     instrs,
		isComplete: isComplete,
	}
}

// Instrs returns a sequence of recorded instructions.
func (t *Trace) Instrs() []ssa.Instruction {
	return t.instrs
}

// IsComplete returns true if the trace is generated from
// a successful run.
func (t *Trace) IsComplete() bool {
	return t.isComplete
}
