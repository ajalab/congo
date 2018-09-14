package congo

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"strings"

	"github.com/ajalab/congo/interp"
	"golang.org/x/tools/go/ssa"

	"github.com/pkg/errors"
)

type Program struct {
	targetPackageName string
	funcName          string
	runnerPackage     *ssa.Package
	targetPackage     *ssa.Package
	symbols           []ssa.Value
}

func (prog *Program) Execute(maxExec uint, minCoverage float64) (*ExecuteResult, error) {
	n := len(prog.symbols)
	values := make([]interface{}, n)
	covered := make(map[*ssa.BasicBlock]struct{})
	targetFunc := prog.targetPackage.Func(prog.funcName)
	var coverage float64
	var symbolValues [][]interface{}

	for i, symbol := range prog.symbols {
		values[i] = zero(symbol.Type())
	}

	for i := uint(0); i < maxExec; i++ {
		result, err := prog.Run(values)
		if err != nil {
			return nil, errors.Wrapf(err, "prog.Execute: failed to run with symbol values %v", values)
		}

		nCoveredBlks := len(covered)
		for _, b := range result.Trace {
			if b.Parent() == targetFunc {
				covered[b] = struct{}{}
			}
		}
		coverage = float64(len(covered)) / float64(len(targetFunc.Blocks))
		log.Println("coverage", coverage)
		if nCoveredBlks < len(covered) {
			symbolValues = append(symbolValues, values)
		}
		if coverage >= minCoverage || i == maxExec-1 {
			break
		}

		solver := NewZ3Solver(prog.symbols, result.Trace)
		queue, queueAfter := make([]int, 0), make([]int, 0)
		for j := len(solver.assertions) - 1; j >= 0; j-- {
			assertion := solver.assertions[j]
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

		for _, j := range queue {
			fmt.Println("negate assertion", j)
			values, err = solver.solve(j)
			if err == nil {
				break
			} else if _, ok := err.(UnsatError); ok {
				log.Println("unsat")
			} else {
				return nil, errors.Wrap(err, "failed to solve assertions")
			}
		}

		solver.Close()
	}
	return &ExecuteResult{
		Coverage:       coverage,
		SymbolValues:   symbolValues,
		targetPackage:  prog.targetPackage.Pkg,
		targetFuncSig:  targetFunc.Signature,
		targetFuncName: targetFunc.Name(),
	}, nil
}

func (prog *Program) Run(values []interface{}) (*interp.CongoInterpResult, error) {
	n := len(values)
	symbolValues := make([]interp.SymbolicValue, n)
	for i, symbol := range prog.symbols {
		symbolValues[i] = interp.SymbolicValue{
			Value: values[i],
			Type:  symbol.Type(),
		}
	}

	// interp.CapturedOutput = new(bytes.Buffer)

	mode := interp.DisableRecover // interp.EnableTracing
	return interp.Interpret(
		prog.runnerPackage,
		prog.targetPackage,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
}

type ExecuteResult struct {
	Coverage     float64
	SymbolValues [][]interface{}

	targetPackage  *types.Package
	targetFuncSig  *types.Signature
	targetFuncName string
}

func (r *ExecuteResult) GenerateTest() error {
	targetPackageName := r.targetPackage.Name()
	targetFuncName := r.targetFuncName
	testFileName := "test.go"
	testFuncName := "Test" + strings.Title(targetFuncName)
	testTemp := fmt.Sprintf(`
		package %s

		import "testing"

		func %s(_ *testing.T) {
			congoTestCases := []struct{}{}
			for _, tc := range congoTestCases {}
		}
	`, targetPackageName, testFuncName)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, testFileName, testTemp, 0)
	if err != nil {
		return errors.Wrap(err, "failed to generate AST for test module")
	}

	targetFuncParams := r.targetFuncSig.Params()
	targetFuncParamsLen := targetFuncParams.Len()
	testFuncDecl := f.Decls[1].(*ast.FuncDecl)
	testCasesExpr := testFuncDecl.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.CompositeLit)
	testCasesType := testCasesExpr.Type.(*ast.ArrayType).Elt.(*ast.StructType)
	testForStmtBody := testFuncDecl.Body.List[1].(*ast.RangeStmt).Body
	testFuncCall := &ast.CallExpr{
		Fun: ast.NewIdent(targetFuncName),
	}
	testForStmtBody.List = append(testForStmtBody.List, &ast.ExprStmt{
		X: testFuncCall,
	})

	for i := 0; i < targetFuncParamsLen; i++ {
		param := targetFuncParams.At(i)
		testCasesType.Fields.List = append(testCasesType.Fields.List, &ast.Field{
			Type:  type2ASTExpr(param.Type()),
			Names: []*ast.Ident{ast.NewIdent(param.Name())},
		})
		testFuncCall.Args = append(testFuncCall.Args, &ast.SelectorExpr{
			X:   ast.NewIdent("tc"),
			Sel: ast.NewIdent(param.Name()),
		})
	}
	for _, values := range r.SymbolValues {
		tc := &ast.CompositeLit{}
		for i := 0; i < targetFuncParamsLen; i++ {
			param := targetFuncParams.At(i)
			tc.Elts = append(tc.Elts, value2ASTExpr(values[i], param.Type()))
		}
		testCasesExpr.Elts = append(testCasesExpr.Elts, tc)
	}

	format.Node(os.Stdout, token.NewFileSet(), f)
	fmt.Printf("%v+", testCasesType)
	return nil
}
