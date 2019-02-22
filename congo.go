package congo

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"io"

	"github.com/ajalab/congo/interp"
	"github.com/ajalab/congo/log"
	"github.com/ajalab/congo/solver"

	"golang.org/x/tools/go/ssa"

	"github.com/pkg/errors"
)

// Program is a type that contains information of the target program.
type Program struct {
	runnerFile         *ast.File
	runnerTypesInfo    *types.Info
	runnerPackage      *ssa.Package
	targetPackage      *ssa.Package
	congoSymbolPackage *ssa.Package
}

// ExecuteOption is a type that contains options to perform concolic execution on a target function.
type ExecuteOption struct {
	MaxExec     uint    `key:"maxexec"`
	MinCoverage float64 `key:"cover"`
}

var defaultExecuteOption = &ExecuteOption{
	MaxExec:     10,
	MinCoverage: 1.0,
}

// Fill fills the fields in ExecuteOption with those in src.
func (eo *ExecuteOption) Fill(src *ExecuteOption, overwrite bool) *ExecuteOption {
	if eo == nil || src == nil {
		return eo
	}

	if overwrite {
		if src.MaxExec != 0 {
			eo.MaxExec = src.MaxExec
		}
		if src.MinCoverage != 0 {
			eo.MinCoverage = src.MinCoverage
		}
	} else {
		if eo.MaxExec == 0 {
			eo.MaxExec = src.MaxExec
		}
		if eo.MinCoverage == 0.0 {
			eo.MinCoverage = src.MinCoverage
		}
	}
	return eo
}

// Target is a type that contains the single target of concolic testing (function and set of symbols).
type Target struct {
	name       string
	f          *ssa.Function
	runnerName string
	symbols    []ssa.Value

	*ExecuteOption
}

// Congo is a type that contains the program and dict of targets
// (keys are names of target function).
type Congo struct {
	program *Program
	targets map[string]*Target
}

// Execute executes concolic execution.
// The iteration time is bounded by maxExec and stopped when minCoverage is accomplished.
func (c *Congo) Execute(funcName string) (*ExecuteResult, error) {
	target, ok := c.targets[funcName]
	if !ok {
		return nil, errors.Errorf("function %s does not exist", funcName)
	}
	n := len(target.symbols)
	solutions := make([]solver.Solution, n)
	covered := make(map[*ssa.BasicBlock]struct{})
	coverage := 0.0
	var runResults []*RunResult

	for i, symbol := range target.symbols {
		solutions[i] = solver.NewIndefinite(symbol.Type())
	}

	for i := uint(0); i < target.MaxExec; i++ {
		values := make([]interface{}, n)
		// Assign a zero value if the concrete value is nil.
		for j, sol := range solutions {
			values[j] = sol.Concretize(zero)
		}

		log.Info.Printf("[%d] run: %v", i, values)

		// Interpret the program with the current symbol values.
		result, err := c.Run(funcName, values)
		if err != nil {
			log.Info.Printf("[%d] panic", i)
		}

		// Update the covered blocks.
		nNewCoveredBlks := 0
		for _, instr := range result.Instrs {
			b := instr.Block()
			if b.Parent() == target.f {
				if _, ok := covered[b]; !ok {
					covered[b] = struct{}{}
					nNewCoveredBlks++
				}
			}
		}

		// Record the concrete values if new blocks are covered.
		if nNewCoveredBlks > 0 {
			runResults = append(runResults, &RunResult{
				symbolValues: values,
				returnValues: result.Return,
				panicked:     result.ExitCode != 0,
			})
		}

		// Compute the coverage and exit if it exceeds the minCoverage.
		// Also exit when the execution count minus one is equal to maxExec to avoid unnecessary constraint solver call.
		coverage = float64(len(covered)) / float64(len(target.f.Blocks))
		log.Info.Printf("[%d] coverage: %.3f", i, coverage)
		if coverage >= target.MinCoverage {
			log.Info.Printf("[%d] stop because the coverage criteria has been satisfied.", i)
			break
		}

		if i == target.MaxExec-1 {
			log.Info.Printf("[%d] stop because the runnign count has reached the limit", i)
		}

		z3Solver, err := solver.CreateZ3Solver(target.symbols, result.Instrs, result.ExitCode == 0)
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
			log.Info.Printf("[%d] negate %d", i, j)
			solutions, err = z3Solver.Solve(j)
			if err == nil {
				log.Info.Printf("[%d] sat %d", i, j)
				break
			} else if _, ok := err.(solver.UnsatError); ok {
				log.Info.Printf("[%d] unsat %d", i, j)
			} else {
				return nil, errors.Wrap(err, "failed to solve assertions")
			}
		}

		z3Solver.Close()
	}

	symbolTypes := make([]types.Type, n)
	for i, symbol := range target.symbols {
		symbolTypes[i] = symbol.Type()
	}

	return &ExecuteResult{
		Coverage:           coverage,
		SymbolTypes:        symbolTypes,
		RunResults:         runResults,
		runnerFile:         c.program.runnerFile,
		runnerTypesInfo:    c.program.runnerTypesInfo,
		runnerPackage:      c.program.runnerPackage.Pkg,
		runnerFuncName:     target.runnerName,
		targetPackage:      c.program.targetPackage.Pkg,
		congoSymbolPackage: c.program.congoSymbolPackage.Pkg,
		targetFuncSig:      target.f.Signature,
		targetFuncName:     funcName,
	}, nil
}

// Run runs the program by the interpreter provided by interp module.
func (c *Congo) Run(funcName string, values []interface{}) (*interp.CongoInterpResult, error) {
	target, ok := c.targets[funcName]
	if !ok {
		return nil, errors.Errorf("function %s does not exist", funcName)
	}
	n := len(values)
	symbolValues := make([]interp.SymbolicValue, n)
	for i, symbol := range target.symbols {
		symbolValues[i] = interp.SymbolicValue{
			Value: values[i],
			Type:  symbol.Type(),
		}
	}

	interp.CapturedOutput = new(bytes.Buffer)
	mode := interp.DisableRecover // interp.EnableTracing
	return interp.Interpret(
		c.program.runnerPackage,
		target.f,
		target.runnerName,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{},
	)
}

// DumpRunnerAST dumps the runner AST file into dest.
func (c *Congo) DumpRunnerAST(dest io.Writer) error {
	return format.Node(dest, token.NewFileSet(), c.program.runnerFile)
}

// DumpSSA dumps the SSA-format code into dest.
func (c *Congo) DumpSSA(dest io.Writer) error {
	var err error
	_, err = c.program.runnerPackage.Func("main").WriteTo(dest)
	if err != nil {
		return err
	}

	for _, target := range c.targets {
		_, err = target.f.WriteTo(dest)
		if err != nil {
			break
		}
	}
	return err
}

// ExecuteResult is a type that contains the result of Execute.
// TODO(ajalab):
// ReturnValues has type []interp.value so it is meaningless to make this property public.
// We use reflection to extract values from interp.value for now.
type ExecuteResult struct {
	Coverage    float64 // achieved coverage.
	SymbolTypes []types.Type
	RunResults  []*RunResult

	runnerFile         *ast.File
	runnerTypesInfo    *types.Info
	runnerPackage      *types.Package
	runnerFuncName     string
	targetPackage      *types.Package
	congoSymbolPackage *types.Package
	targetFuncSig      *types.Signature
	targetFuncName     string
}

// RunResult is a type that contains the result of Run.
type RunResult struct {
	symbolValues []interface{}
	returnValues interface{}
	panicked     bool
}
