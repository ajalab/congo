package solver

import "golang.org/x/tools/go/ssa"

// IfBranch contains a branching instruction (*ssa.If) and
// the direction taken in the concolic execution.
type IfBranch struct {
	Instr     *ssa.If
	Direction bool
}

// UnsatError is an error describing that Z3 constraints were unsatisfied.
type UnsatError struct{}

func (ue UnsatError) Error() string {
	return "unsat"
}
