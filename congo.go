package congo

import (
	"bytes"
	"go/format"
	"go/token"
	"go/types"
	"io"
	"log"

	"github.com/ajalab/congo/interp"
	"github.com/ajalab/congo/solver"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"

	"github.com/pkg/errors"
)

// Program is a type that contains information of the target program and symbols.
type Program struct {
	runnerPackageInfo  *loader.PackageInfo
	runnerPackage      *ssa.Package
	targetPackage      *ssa.Package
	congoSymbolPackage *ssa.Package
	targetFunc         *ssa.Function
	symbols            []ssa.Value
}

// Execute executes concolic execution.
// The iteration time is bounded by maxExec and stopped when minCoverage is accomplished.
func (prog *Program) Execute(maxExec uint, minCoverage float64) (*ExecuteResult, error) {
	n := len(prog.symbols)
	values := make([]interface{}, n)
	var symbolValues [][]interface{}
	var returnValues []interface{}
	covered := make(map[*ssa.BasicBlock]struct{})
	coverage := 0.0

	for i := uint(0); i < maxExec; i++ {
		// Assign a zero value if the concrete value is nil.
		for j, symbol := range prog.symbols {
			if values[j] == nil {
				values[j] = zero(symbol.Type())
			}
		}

		// Interpret the program with the current symbol values.
		result, err := prog.Run(values)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to run with symbol values %v", values)
		}

		// Update the covered blocks.
		nNewCoveredBlks := 0
		for _, instr := range result.Trace {
			b := instr.Block()
			if b.Parent() == prog.targetFunc {
				if _, ok := covered[b]; !ok {
					covered[b] = struct{}{}
					nNewCoveredBlks++
				}
			}
		}
		// Record the symbol values if new blocks are covered.
		if nNewCoveredBlks > 0 {
			symbolValues = append(symbolValues, values)
			returnValues = append(returnValues, result.ReturnValue)
		}

		// Compute the coverage and exit if it exceeds the minCoverage.
		// Also exit when the execution count minus one is equal to maxExec to avoid unnecessary constraint solver call.
		coverage = float64(len(covered)) / float64(len(prog.targetFunc.Blocks))
		log.Println("coverage", coverage)
		if coverage >= minCoverage || i == maxExec-1 {
			break
		}

		z3Solver := solver.NewZ3Solver()
		err = z3Solver.LoadSymbols(prog.symbols)
		if err != nil {
			return nil, err
		}
		z3Solver.LoadTrace(result.Trace)
		queue, queueAfter := make([]int, 0), make([]int, 0)
		for j := z3Solver.NumBranches() - 1; j >= 0; j-- {
			if branch, ok := z3Solver.Branch(j).(*solver.BranchIf); ok {
				succs := branch.Succs()
				b := succs[0]
				if branch.Direction {
					b = succs[1]
				}
				if _, ok := covered[b]; !ok {
					queue = append(queue, j)
				} else {
					queueAfter = append(queueAfter, j)
				}
			}
		}
		queue = append(queue, queueAfter...)

		for _, j := range queue {
			values, err = z3Solver.Solve(j)
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
		runnerPackageInfo:  prog.runnerPackageInfo,
		targetPackage:      prog.targetPackage.Pkg,
		congoSymbolPackage: prog.congoSymbolPackage.Pkg,
		targetFuncSig:      prog.targetFunc.Signature,
		targetFuncName:     prog.targetFunc.Name(),
	}, nil
}

// Run runs the program by the interpreter provided by interp module.
func (prog *Program) Run(values []interface{}) (*interp.CongoInterpResult, error) {
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
	return interp.Interpret(
		prog.runnerPackage,
		prog.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
}

// DumpRunnerAST dumps the runner AST file into dest.
func (prog *Program) DumpRunnerAST(dest io.Writer) error {
	return format.Node(dest, token.NewFileSet(), prog.runnerPackageInfo.Files[0])
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

	runnerPackageInfo  *loader.PackageInfo
	targetPackage      *types.Package
	congoSymbolPackage *types.Package
	targetFuncSig      *types.Signature
	targetFuncName     string
}
