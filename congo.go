package congo

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"io"
	"log"

	"github.com/ajalab/congo/interp"
	"github.com/ajalab/congo/solver"
	"github.com/ajalab/congo/trace"

	"golang.org/x/tools/go/ssa"

	"github.com/pkg/errors"
)

// Program is a type that contains information of the target program and symbols.
type Program struct {
	runnerFile         *ast.File
	runnerTypesInfo    *types.Info
	runnerPackage      *ssa.Package
	targetPackage      *ssa.Package
	congoSymbolPackage *ssa.Package
	targetFunc         *ssa.Function
	symbols            []ssa.Value
}

// RunResult contains a running trace and information such as
// whether it is a complete trace without panic.
type RunResult struct {
	Trace        *trace.Trace
	ReturnValues interface{}
}

// Execute executes concolic execution.
// The iteration time is bounded by maxExec and stopped when minCoverage is accomplished.
func (prog *Program) Execute(maxExec uint, minCoverage float64) (*ExecuteResult, error) {
	n := len(prog.symbols)
	solutions := make([]solver.Solution, n)
	var symbolValues [][]interface{}
	var returnValues []interface{}
	covered := make(map[*ssa.BasicBlock]struct{})
	coverage := 0.0

	for i, symbol := range prog.symbols {
		solutions[i] = solver.NewIndefinite(symbol.Type())
	}

	for i := uint(0); i < maxExec; i++ {
		values := make([]interface{}, n)
		// Assign a zero value if the concrete value is nil.
		for j, sol := range solutions {
			values[j] = sol.Concretize(zero)
		}

		// Interpret the program with the current symbol values.
		result, err := prog.Run(values)
		if err != nil {
			log.Printf("panic occurred: %v", values)
		}

		// Update the covered blocks.
		nNewCoveredBlks := 0
		for _, instr := range result.Trace.Instrs() {
			b := instr.Block()
			if b.Parent() == prog.targetFunc {
				if _, ok := covered[b]; !ok {
					covered[b] = struct{}{}
					nNewCoveredBlks++
				}
			}
		}
		// Record the concrete values if new blocks are covered.
		if nNewCoveredBlks > 0 {
			symbolValues = append(symbolValues, values)
			returnValues = append(returnValues, result.ReturnValues)
		}

		// Compute the coverage and exit if it exceeds the minCoverage.
		// Also exit when the execution count minus one is equal to maxExec to avoid unnecessary constraint solver call.
		coverage = float64(len(covered)) / float64(len(prog.targetFunc.Blocks))
		log.Println("coverage", coverage)
		if coverage >= minCoverage || i == maxExec-1 {
			log.Println("stop")
			break
		}

		z3Solver, err := solver.CreateZ3Solver(prog.symbols, result.Trace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a solver")
		}

		branches := z3Solver.Branches()
		queue, queueAfter := make([]int, 0), make([]int, 0)
		for j := len(branches) - 1; j >= 0; j-- {
			branch := branches[j]
			switch branch := branch.(type) {
			case *solver.BranchIf:
				b := branch.Other()
				if _, ok := covered[b]; !ok {
					queue = append(queue, j)
				} else {
					queueAfter = append(queueAfter, j)
				}
			case *solver.BranchDeref:
				queue = append(queue, j)
			}
		}
		queue = append(queue, queueAfter...)

		for _, j := range queue {
			solutions, err = z3Solver.Solve(j)
			if err == nil {
				break
			} else if _, ok := err.(solver.UnsatError); ok {
				log.Println("unsat")
			} else {
				return nil, errors.Wrap(err, "failed to solve assertions")
			}
		}

		z3Solver.Close()
	}

	symbolTypes := make([]types.Type, n)
	for i, symbol := range prog.symbols {
		symbolTypes[i] = symbol.Type()
	}

	return &ExecuteResult{
		Coverage:           coverage,
		SymbolValues:       symbolValues,
		SymbolTypes:        symbolTypes,
		ReturnValues:       returnValues,
		runnerFile:         prog.runnerFile,
		runnerTypesInfo:    prog.runnerTypesInfo,
		runnerPackage:      prog.runnerPackage.Pkg,
		targetPackage:      prog.targetPackage.Pkg,
		congoSymbolPackage: prog.congoSymbolPackage.Pkg,
		targetFuncSig:      prog.targetFunc.Signature,
		targetFuncName:     prog.targetFunc.Name(),
	}, nil
}

// Run runs the program by the interpreter provided by interp module.
func (prog *Program) Run(values []interface{}) (*RunResult, error) {
	n := len(values)
	symbolValues := make([]interp.SymbolicValue, n)
	for i, symbol := range prog.symbols {
		symbolValues[i] = interp.SymbolicValue{
			Value: values[i],
			Type:  symbol.Type(),
		}
	}

	interp.CapturedOutput = new(bytes.Buffer)
	mode := interp.DisableRecover // interp.EnableTracing
	interpResult, err := interp.Interpret(
		prog.runnerPackage,
		prog.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{},
	)

	return &RunResult{
		Trace: trace.NewTrace(
			interpResult.Instrs,
			interpResult.ExitCode == 0,
		),
		ReturnValues: interpResult.ReturnValue,
	}, err
}

// DumpRunnerAST dumps the runner AST file into dest.
func (prog *Program) DumpRunnerAST(dest io.Writer) error {
	return format.Node(dest, token.NewFileSet(), prog.runnerFile)
}

// DumpRunnerSSA dumps the runner SSA into dest.
func (prog *Program) DumpRunnerSSA(dest io.Writer) error {
	_, err := prog.runnerPackage.Func("main").WriteTo(dest)
	return err
}

// ExecuteResult is a type that contains the result of Program.Execute.
// TODO(ajalab):
// ReturnValues has type []interp.value so it is meaningless to make this property public.
// We use reflection to extract values from interp.value for now.
type ExecuteResult struct {
	Coverage     float64         // achieved coverage.
	SymbolValues [][]interface{} // list of values for symbols.
	SymbolTypes  []types.Type
	ReturnValues []interface{} // returned values corresponding to execution results. (invariant: len(SymbolValues) == len(ReturnValues))

	runnerFile         *ast.File
	runnerTypesInfo    *types.Info
	runnerPackage      *types.Package
	targetPackage      *types.Package
	congoSymbolPackage *types.Package
	targetFuncSig      *types.Signature
	targetFuncName     string
}
