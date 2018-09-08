package congo

import (
	"fmt"
	"go/types"
	"log"

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

type ExecuteResult struct {
	Coverage float64
}

func (prog *Program) Execute(maxExec uint, minCoverage float64) (*ExecuteResult, error) {
	n := len(prog.symbols)
	symbolValues := make([]interp.SymbolicValue, n)
	covered := make(map[*ssa.BasicBlock]struct{})
	targetFunc := prog.targetPackage.Func(prog.funcName)
	var coverage float64

	for i := 0; i < n; i++ {
		ty := prog.symbols[i].Type()
		symbolValues[i] = interp.SymbolicValue{
			Value: zero(ty),
			Type:  ty,
		}
	}

	for i := uint(0); i < maxExec; i++ {
		fmt.Println(symbolValues)
		traces, err := prog.Run(symbolValues)
		if err != nil {
			return nil, err
		}

		for _, trace := range traces {
			for _, b := range trace {
				if b.Parent() == targetFunc {
					covered[b] = struct{}{}
					fmt.Printf("%s ", b)
				}
			}
		}
		fmt.Println()
		coverage = float64(len(covered)) / float64(len(targetFunc.Blocks))
		fmt.Println("coverage", coverage)
		if coverage >= minCoverage || i == maxExec-1 {
			break
		}

		cs := fromTrace(prog.symbols, traces)
		queue, queueAfter := make([]int, 0), make([]int, 0)
		for j := len(cs.assertions) - 1; j >= 0; j-- {
			assertion := cs.assertions[j]
			succs := assertion.instr.Block().Succs
			b := succs[0]
			if assertion.orig {
				b = succs[1]
			}
			if _, ok := covered[b]; !ok {
				queue = append(queue, j)
			} else {
				queueAfter = append(queueAfter, j)
			}
		}
		queue = append(queue, queueAfter...)

		var values []interface{}
		for _, j := range queue {
			fmt.Println("negate assertion", j)
			values, err = cs.solve(j)
			if err == nil {
				break
			} else if _, ok := err.(UnsatError); ok {
				log.Println("unsat")
			} else {
				return nil, err
			}
		}
		for j, v := range values {
			symbolValues[j].Value = v
		}

		cs.Close()
	}

	return &ExecuteResult{Coverage: coverage}, nil
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
