package congo

import (
	"fmt"

	"golang.org/x/tools/go/ssa"
)

type constraintsSet struct {
}

type constraint struct {
}

func fromTrace(trace [][]*ssa.BasicBlock) *constraintsSet {
	for _, blocks := range trace {
		for _, block := range blocks {
			fmt.Printf(".%d:\n", block.Index)
			for _, instr := range block.Instrs {
				fmt.Printf("%[1]v: %[1]T\n", instr)
				switch instr := instr.(type) {
				case *ssa.BinOp:
					fmt.Printf("\t%v\n\t%v\n", instr.X, instr.Y)
				}
			}
		}
	}
	return nil
}
