package congo

import (
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

func (prog *Program) Execute() error {
	n := len(prog.symbols)
	symbolValues := make([]interp.SymbolicValue, n)

	for i := 0; i < n; i++ {
		ty := prog.symbols[i].Type()
		symbolValues[i] = interp.SymbolicValue{
			Value: zero(ty),
			Type:  ty,
		}
	}

	for i := 0; i < 3; i++ {
		traces, err := prog.Run(symbolValues)
		if err != nil {
			return err
		}
		cs := fromTrace(prog.symbols, traces)
		values, err := cs.solve(len(cs.assertions) - 1)
		if err != nil {
			cs.Close()
			return err
		}
		for j, v := range values {
			symbolValues[j].Value = v
		}

		cs.Close()
	}

	return nil
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
