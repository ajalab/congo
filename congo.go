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
	runnerPackage *ssa.Package
	targetPackage *ssa.Package
	targetFunc    *ssa.Function
	symbols       []ssa.Value
}

func (prog *Program) Execute(maxExec uint, minCoverage float64) (*ExecuteResult, error) {
	n := len(prog.symbols)
	values := make([]interface{}, n)
	covered := make(map[*ssa.BasicBlock]struct{})
	var coverage float64
	var symbolValues [][]interface{}
	var returnValues []interface{}

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
			if b.Parent() == prog.targetFunc {
				covered[b] = struct{}{}
			}
		}
		coverage = float64(len(covered)) / float64(len(prog.targetFunc.Blocks))
		log.Println("coverage", coverage)
		if nCoveredBlks < len(covered) {
			symbolValues = append(symbolValues, values)
			returnValues = append(returnValues, result.ReturnValue)
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
		ReturnValues:   returnValues,
		targetPackage:  prog.targetPackage.Pkg,
		targetFuncSig:  prog.targetFunc.Signature,
		targetFuncName: prog.targetFunc.Name(),
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
		prog.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
}

type ExecuteResult struct {
	Coverage     float64
	SymbolValues [][]interface{}
	ReturnValues []interface{}

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

	// Add fields to the struct type for test cases (testCasesType)
	// Add arguments to the function call expression
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

	// Add test cases
	for _, values := range r.SymbolValues {
		//fmt.Printf("%[1]v: %[1]T\n", reflect.ValueOf(r.ReturnValues[i]).Index(0).Interface())
		tc := &ast.CompositeLit{}
		for j := 0; j < targetFuncParamsLen; j++ {
			param := targetFuncParams.At(j)
			tc.Elts = append(tc.Elts, value2ASTExpr(values[j], param.Type()))
		}
		testCasesExpr.Elts = append(testCasesExpr.Elts, tc)
	}

	targetFuncRetLen := r.targetFuncSig.Results().Len()
	switch targetFuncRetLen {
	case 0:
		testForStmtBody.List = append(testForStmtBody.List, &ast.ExprStmt{
			X: testFuncCall,
		})
	case 1:
		testForStmtBody.List = append(testForStmtBody.List, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("actual")},
			Rhs: []ast.Expr{testFuncCall},
			Tok: token.DEFINE,
		})
		retTy := r.targetFuncSig.Results().At(0).Type()
		testCasesType.Fields.List = append(testCasesType.Fields.List, &ast.Field{
			Type:  type2ASTExpr(retTy),
			Names: []*ast.Ident{ast.NewIdent("expected")},
		})
		for i, v := range r.ReturnValues {
			tc := testCasesExpr.Elts[i].(*ast.CompositeLit)
			tc.Elts = append(tc.Elts, value2ASTExpr(v, retTy))
		}
	default:
		actualVars := make([]ast.Expr, targetFuncRetLen)
		for i := 0; i < targetFuncRetLen; i++ {
			actualVars[i] = ast.NewIdent(fmt.Sprintf("actual%d", i))
		}
		testForStmtBody.List = append(testForStmtBody.List, &ast.AssignStmt{
			Lhs: actualVars,
			Rhs: []ast.Expr{testFuncCall},
			Tok: token.DEFINE,
		})
	}

	format.Node(os.Stdout, token.NewFileSet(), f)
	return nil
}
