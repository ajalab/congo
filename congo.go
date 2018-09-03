package congo

import (
	"fmt"
	"go/types"

	"github.com/ajalab/congo/interp"
	"golang.org/x/tools/go/ssa"
)

type Program struct {
	targetPackageName string
	funcName          string
	runnerPackage     *ssa.Package
	targetPackage     *ssa.Package
	symbols           []ssa.Value
}

func (prog *Program) Execute(maxExec uint, minCoverage float64) error {
	n := len(prog.symbols)
	symbolValues := make([]interp.SymbolicValue, n)
	covered := make(map[*ssa.BasicBlock]struct{})
	targetFunc := prog.targetPackage.Func(prog.funcName)

	for i := 0; i < n; i++ {
		ty := prog.symbols[i].Type()
		symbolValues[i] = interp.SymbolicValue{
			Value: zero(ty),
			Type:  ty,
		}
	}

	for i := uint(0); i < maxExec; i++ {
		traces, err := prog.Run(symbolValues)
		if err != nil {
			return err
		}

		for _, trace := range traces {
			for _, b := range trace {
				if b.Parent() == targetFunc {
					covered[b] = struct{}{}
				}
			}
		}
		coverage := float64(len(covered)) / float64(len(targetFunc.Blocks))
		fmt.Println("coverage", coverage)
		if coverage >= minCoverage {
			break
		}

		cs := fromTrace(prog.symbols, traces)
		values, err := cs.solve(len(cs.assertions) - 1)
		if err != nil {
			cs.Close()
			return err
		}
		fmt.Println(values)
		for j, v := range values {
			symbolValues[j].Value = v
		}

		cs.Close()
	}

	return nil
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
