package congo

import (
	"fmt"
	"go/types"
	"os"

	"github.com/ajalab/congo/interp"
	"golang.org/x/tools/go/ssa"
)

type Program struct {
	targetPackageName string
	funcName          string
	runnerPackage     *ssa.Package
	targetPackage     *ssa.Package
	mainFunc          *ssa.Function
	symbols           []ssa.Value
}

func (prog *Program) Dump() {
	prog.targetPackage.Func(prog.funcName).WriteTo(os.Stdout)
}

func (prog *Program) Execute() {
	n := len(prog.symbols)
	symbolValues := make([]interp.SymbolicValue, n)

	for i := 0; i < n; i++ {
		ty := prog.symbols[i].Type()
		symbolValues[i] = interp.SymbolicValue{
			Value: zero(ty),
			Type:  ty,
		}
	}

	traces, _ := prog.Run(symbolValues)

	// TODO(ajalab) Change params to *ssa.TypeAssert instead of *ssa.Parameter
	/*
		params := []ssa.Value{}
		targetFunc := prog.targetPackage.Func(prog.funcName)
		for _, param := range targetFunc.Params {
			params = append(params, param)
		}
	*/

	cs := fromTrace(prog.symbols, traces)
	defer cs.Close()

	cs.solve(len(cs.assertions) - 1)
	fmt.Println(traces)
}

func (prog *Program) RunWithZeroValues() ([][]*ssa.BasicBlock, error) {
	n := len(prog.symbols)
	symbolValues := make([]interp.SymbolicValue, n)
	for i := 0; i < n; i++ {
		ty := prog.symbols[i].Type()
		symbolValues[i] = interp.SymbolicValue{
			Value: zero(ty),
			Type:  ty,
		}
	}

	return prog.Run(symbolValues)
}

func (prog *Program) Run(symbolValues []interp.SymbolicValue) ([][]*ssa.BasicBlock, error) {
	mode := interp.DisableRecover // interp.EnableTracing
	trace, _ := interp.Interpret(
		prog.runnerPackage,
		prog.targetPackage,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
	return trace, nil
}
