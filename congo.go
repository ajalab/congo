package congo

import (
	"fmt"
	"go/types"
	"os"

	"github.com/ajalab/congo/interp"
	"golang.org/x/tools/go/ssa"
)

type Program struct {
	packageName   string
	funcName      string
	packageRunner *ssa.Package
	mainFunc      *ssa.Function
	targetFunc    *ssa.Function
	symbols       []types.Type
}

func (prog *Program) Dump() {
	prog.targetFunc.WriteTo(os.Stdout)
}

func (prog *Program) Execute() {
	traces, _ := prog.RunWithZeroValues()
	fromTrace(prog.targetFunc, traces[0])
	fmt.Println(traces)
}

func (prog *Program) RunWithZeroValues() ([][]*ssa.BasicBlock, error) {
	n := len(prog.symbols)
	symbolValues := make([]interp.SymbolicValue, n)
	for i := 0; i < n; i++ {
		ty := prog.symbols[i]
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
		prog.packageRunner,
		prog.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
	return trace, nil
}
