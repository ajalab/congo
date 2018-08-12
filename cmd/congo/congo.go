package main

import (
	"fmt"
	"go/types"

	"github.com/ajalab/congo/cmd/congo/interp"
	"golang.org/x/tools/go/ssa"
)

type program struct {
	packageName   string
	funcName      string
	packageRunner *ssa.Package
	mainFunc      *ssa.Function
	targetFunc    *ssa.Function
	symbols       []types.Type
}

func (prog *program) RunWithZeroValues() error {
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

func (prog *program) Run(symbolValues []interp.SymbolicValue) error {
	mode := interp.DisableRecover // interp.EnableTracing
	trace, _ := interp.Interpret(
		prog.packageRunner,
		prog.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
	fmt.Println("trace", trace)
	return nil
}
