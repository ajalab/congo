package main

import (
	"go/types"
	"os"

	"github.com/ajalab/congo/cmd/congo/interp"
	"golang.org/x/tools/go/ssa"
)

type Program struct {
	PackageName   string
	FuncName      string
	packageRunner *ssa.Package
	mainFunc      *ssa.Function
	targetFunc    *ssa.Function
	symbols       []types.Type
}

func (program *Program) Run() error {
	n := len(program.symbols)
	symbolValues := make([]interp.SymbolicValue, n)
	for i := 0; i < n; i++ {
		symbolValues[i] = interp.SymbolicValue{Value: int32(0), Type: program.symbols[i]}
	}

	program.mainFunc.WriteTo(os.Stdout)

	mode := interp.DisableRecover // interp.EnableTracing
	interp.Interpret(
		program.packageRunner,
		program.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
	return nil
}
