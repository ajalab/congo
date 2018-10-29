package congo

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/ast/astutil"
)

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
