package congo

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"reflect"
	"strings"

	"github.com/ajalab/congo/interp"
	"github.com/ajalab/congo/solver"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"

	"github.com/pkg/errors"
)

// Program is a type that contains information of the target program and symbols.
type Program struct {
	runnerPackageInfo  *loader.PackageInfo
	runnerPackage      *ssa.Package
	targetPackage      *ssa.Package
	congoSymbolPackage *ssa.Package
	targetFunc         *ssa.Function
	symbols            []ssa.Value
}

// Execute executes concolic execution.
// The iteration time is bounded by maxExec and stopped when minCoverage is accomplished.
func (prog *Program) Execute(maxExec uint, minCoverage float64) (*ExecuteResult, error) {
	n := len(prog.symbols)
	values := make([]interface{}, n)
	var symbolValues [][]interface{}
	var returnValues []interface{}
	covered := make(map[*ssa.BasicBlock]struct{})
	coverage := 0.0

	for i := uint(0); i < maxExec; i++ {
		// Assign a zero value if the concrete value is nil.
		for j, symbol := range prog.symbols {
			if values[j] == nil {
				values[j] = zero(symbol.Type())
			}
		}

		// Interpret the program with the current symbol values.
		// TODO(ajalab) handle panic occurred in the target
		result, err := prog.Run(values)
		if err != nil {
			return nil, errors.Wrapf(err, "prog.Execute: failed to run with symbol values %v", values)
		}

		// Update the covered blocks.
		nNewCoveredBlks := 0
		for _, instr := range result.Trace {
			b := instr.Block()
			if b.Parent() == prog.targetFunc {
				if _, ok := covered[b]; !ok {
					covered[b] = struct{}{}
					nNewCoveredBlks++
				}
			}
		}
		// Record the symbol values if new blocks are covered.
		if nNewCoveredBlks > 0 {
			symbolValues = append(symbolValues, values)
			returnValues = append(returnValues, result.ReturnValue)
		}

		// Compute the coverage and exit if it exceeds the minCoverage.
		// Also exit when the execution count minus one is equal to maxExec to avoid unnecessary constraint solver call.
		coverage = float64(len(covered)) / float64(len(prog.targetFunc.Blocks))
		log.Println("coverage", coverage)
		if coverage >= minCoverage || i == maxExec-1 {
			break
		}

		z3Solver := solver.NewZ3Solver()
		err = z3Solver.LoadSymbols(prog.symbols)
		if err != nil {
			return nil, err
		}
		z3Solver.LoadTrace(result.Trace)
		queue, queueAfter := make([]int, 0), make([]int, 0)
		for j := z3Solver.NumBranches() - 1; j >= 0; j-- {
			branch := z3Solver.Branch(j)
			succs := branch.Instr.Block().Succs
			b := succs[0]
			if branch.Direction {
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
			values, err = z3Solver.Solve(j)
			if err == nil {
				break
			} else if _, ok := err.(solver.UnsatError); ok {
				log.Println("unsat")
			} else {
				return nil, errors.Wrap(err, "failed to solve assertions")
			}
		}

		z3Solver.Close()
	}

	symbolTypes := make([]types.Type, n)
	for i, symbol := range prog.symbols {
		symbolTypes[i] = symbol.Type()
	}

	return &ExecuteResult{
		Coverage:           coverage,
		SymbolValues:       symbolValues,
		SymbolTypes:        symbolTypes,
		ReturnValues:       returnValues,
		runnerPackageInfo:  prog.runnerPackageInfo,
		targetPackage:      prog.targetPackage.Pkg,
		congoSymbolPackage: prog.congoSymbolPackage.Pkg,
		targetFuncSig:      prog.targetFunc.Signature,
		targetFuncName:     prog.targetFunc.Name(),
	}, nil
}

// Run runs the program by the interpreter provided by interp module.
func (prog *Program) Run(values []interface{}) (*interp.CongoInterpResult, error) {
	n := len(values)
	symbolValues := make([]interp.SymbolicValue, n)
	for i, symbol := range prog.symbols {
		symbolValues[i] = interp.SymbolicValue{
			Value: values[i],
			Type:  symbol.Type(),
		}
	}

	interp.CapturedOutput = new(bytes.Buffer)
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

// DumpRunnerAST dumps the runner AST file into dest.
func (prog *Program) DumpRunnerAST(dest io.Writer) error {
	return format.Node(dest, token.NewFileSet(), prog.runnerPackageInfo.Files[0])
}

// DumpRunnerSSA dumps the runner SSA into dest.
func (prog *Program) DumpRunnerSSA(dest io.Writer) error {
	_, err := prog.runnerPackage.Func("main").WriteTo(dest)
	return err
}

// ExecuteResult is a type that contains the result of Program.Execute.
// TODO(ajalab):
// ReturnValues has type []interp.value so it is meaningless to make this property public.
// We use reflection to extract values from interp.value for now.
type ExecuteResult struct {
	Coverage     float64         // achieved coverage.
	SymbolValues [][]interface{} // list of values for symbols.
	SymbolTypes  []types.Type
	ReturnValues []interface{} // returned values corresponding to execution results. (invariant: len(SymbolValues) == len(ReturnValues))

	runnerPackageInfo  *loader.PackageInfo
	targetPackage      *types.Package
	congoSymbolPackage *types.Package
	targetFuncSig      *types.Signature
	targetFuncName     string
}

// GenerateTest generates test module for the program.
func (r *ExecuteResult) GenerateTest() (*ast.File, error) {
	runnerFuncName := "main" // TODO(ajalab): parametrize this variable for arbitrary defined runner functions

	// Rewrite symbols (symbol.Symbols and symbol.RetVals) in the runner function
	runnerFunc := r.runnerPackageInfo.Files[0].Scope.Lookup(runnerFuncName).Decl.(*ast.FuncDecl)
	symbolNames, retValNames, err := r.rewriteSymbols(runnerFunc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate test code")
	}

	// Determine the name for the variable of type *testing.T
	testingT := "testingT"
	runnerFuncType := r.runnerPackageInfo.Pkg.Scope().Lookup(runnerFuncName).(*types.Func)
	if runnerFuncType.Scope().Lookup("t") == nil {
		testingT = "t"
	}

	// Rewrite congo assertions
	err = r.rewriteAssertions(testingT, runnerFunc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate test code")
	}

	// Now we prepare the AST file for test to generate
	testFuncName := "Test" + strings.Title(r.targetFuncName)
	testTemp := fmt.Sprintf(`
		package %s

		func %s(%s *testing.T) {
			congoTestCases := []struct{}{}
			for _, tc := range congoTestCases {}
		}
	`, r.targetPackage.Name()+"_test", testFuncName, testingT)

	fset := token.NewFileSet()
	testFileName := "test.go"
	f, err := parser.ParseFile(fset, testFileName, testTemp, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate AST for test module")
	}
	astutil.AddImport(fset, f, "testing")
	astutil.AddImport(fset, f, r.targetPackage.Path())

	// Add symbol value fields to the struct type for test cases (testCasesType)
	testFuncDecl := f.Scope.Lookup(testFuncName).Decl.(*ast.FuncDecl)
	testCasesExpr := testFuncDecl.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.CompositeLit)
	testCasesType := testCasesExpr.Type.(*ast.ArrayType).Elt.(*ast.StructType)
	for i, name := range symbolNames {
		testCasesType.Fields.List = append(testCasesType.Fields.List, &ast.Field{
			Type:  type2ASTExpr(r.SymbolTypes[i]),
			Names: []*ast.Ident{ast.NewIdent(name)},
		})
	}

	// Add oracle value fields to the struct type for test cases (testCasesType)
	for i, name := range retValNames {
		testCasesType.Fields.List = append(testCasesType.Fields.List, &ast.Field{
			Type:  type2ASTExpr(r.targetFuncSig.Results().At(i).Type()),
			Names: []*ast.Ident{ast.NewIdent(name)},
		})
	}

	// Add test cases
	for i, symbolValues := range r.SymbolValues {
		// Add symbol values
		tc := &ast.CompositeLit{}
		for j, value := range symbolValues {
			ty := r.SymbolTypes[j]
			tc.Elts = append(tc.Elts, value2ASTExpr(value, ty))
		}

		// Add oracle values
		returnValues := r.ReturnValues[i]
		returnValuesLen := r.targetFuncSig.Results().Len()
		switch {
		case returnValuesLen == 1:
			value := reflect.ValueOf(returnValues).Interface()
			ty := r.targetFuncSig.Results().At(0).Type()
			tc.Elts = append(tc.Elts, value2ASTExpr(value, ty))
		case returnValuesLen >= 2:
			for j := 0; j < returnValuesLen; j++ {
				value := reflect.ValueOf(returnValues).Index(j).Interface()
				ty := r.targetFuncSig.Results().At(j).Type()
				tc.Elts = append(tc.Elts, value2ASTExpr(value, ty))
			}
		}

		testCasesExpr.Elts = append(testCasesExpr.Elts, tc)
	}
	testRangeStmtBody := testFuncDecl.Body.List[1].(*ast.RangeStmt).Body
	testRangeStmtBody.List = runnerFunc.Body.List

	return f, nil
}

func (r *ExecuteResult) rewriteSymbols(runnerFunc *ast.FuncDecl) ([]string, []string, error) {
	symbolType := r.congoSymbolPackage.Scope().Lookup("SymbolType").Type()
	retValType := r.congoSymbolPackage.Scope().Lookup("RetValType").Type()
	var err error

	symbolNames := make([]string, len(r.SymbolValues[0]))
	for i := range symbolNames {
		symbolNames[i] = fmt.Sprintf("symbol%d", i)
	}
	retValNames := make([]string, r.targetFuncSig.Results().Len())
	if len(retValNames) == 1 {
		retValNames[0] = "expected"
	} else {
		for i := range retValNames {
			retValNames[i] = fmt.Sprintf("expected%d", i)
		}
	}

	astutil.Apply(runnerFunc, func(c *astutil.Cursor) bool {
		// Search for type assertions expression e[i].(type) which satisfies the following requirements
		// 1. e[i] has type symbol.SymbolType or symbolRetValType
		// 2. i is a constant value
		node := c.Node()
		typeAssertExpr, ok := node.(*ast.TypeAssertExpr)
		if !ok {
			return true
		}
		indexExpr, ok := typeAssertExpr.X.(*ast.IndexExpr)
		if !ok {
			return true
		}
		ty := r.runnerPackageInfo.TypeOf(indexExpr)
		if !(ty == symbolType || ty == retValType) {
			return true
		}
		indexTV, ok := r.runnerPackageInfo.Types[indexExpr.Index]
		if !(ok && indexTV.Value.Kind() == constant.Int) {
			err = errors.New("indexing symbols should be constant")
			return false
		}
		i, _ := constant.Int64Val(indexTV.Value)
		var name string
		switch ty {
		case symbolType:
			if callExpr, ok := c.Parent().(*ast.CallExpr); ok {
				sig := r.runnerPackageInfo.TypeOf(callExpr.Fun).(*types.Signature)
				symbolNames[i] = sig.Params().At(c.Index()).Name()
			}
			name = symbolNames[i]
		case retValType:
			r := r.targetFuncSig.Results().At(int(i))
			if n := r.Name(); n != "" {
				retValNames[i] = n
			}
			name = retValNames[i]
		}
		c.Replace(&ast.SelectorExpr{
			X:   ast.NewIdent("tc"),
			Sel: ast.NewIdent(name),
		})
		return false
	}, nil)
	return symbolNames, retValNames, err
}

func (r *ExecuteResult) rewriteAssertions(testingT string, runnerFunc *ast.FuncDecl) error {
	testAssertType := r.congoSymbolPackage.Scope().Lookup("TestAssert").Type()
	astutil.Apply(runnerFunc, func(c *astutil.Cursor) bool {
		node := c.Node()
		exprStmt, ok := node.(*ast.ExprStmt)
		if !ok {
			return true
		}
		callExpr, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		funcType := r.runnerPackageInfo.TypeOf(callExpr.Fun)
		if funcType == testAssertType {
			cond := callExpr.Args[0]
			assertion := &ast.IfStmt{
				Cond: negateCond(cond),
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.ExprStmt{
							X: &ast.CallExpr{
								Fun: &ast.SelectorExpr{
									X:   ast.NewIdent(testingT),
									Sel: ast.NewIdent("Error"),
								},
								Args: []ast.Expr{&ast.BasicLit{
									Kind:  token.STRING,
									Value: "\"assertion failed\"",
								}},
							},
						},
					},
				},
			}
			c.Replace(assertion)
			return false
		}
		return true
	}, nil)
	return nil
}

func negateCond(cond ast.Expr) ast.Expr {
	if binCond, ok := cond.(*ast.BinaryExpr); ok {
		newOp := token.ILLEGAL
		switch binCond.Op {
		case token.EQL:
			newOp = token.NEQ
		case token.NEQ:
			newOp = token.EQL
		}
		if newOp != token.ILLEGAL {
			return &ast.BinaryExpr{
				X:  binCond.X,
				Y:  binCond.Y,
				Op: newOp,
			}
		}
	}
	return &ast.UnaryExpr{
		Op: token.NOT,
		X:  cond,
	}
}
