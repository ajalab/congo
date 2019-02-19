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
	// Rewrite symbols (symbol.Symbols and symbol.RetVals) in the runner function
	runnerFunc := r.runnerFile.Scope.Lookup(r.runnerFuncName).Decl.(*ast.FuncDecl)
	symbolNames, retValNames, err := r.rewriteSymbols(runnerFunc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate test code")
	}

	// Determine the name for the variable of type *testing.T
	testingT := "testingT"
	runnerFuncType := r.runnerPackage.Scope().Lookup(r.runnerFuncName).(*types.Func)
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

		func %s(t *testing.T) {
			congoTestCases := []struct{}{}
			for i, tc := range congoTestCases {
				t.Run(fmt.Sprintf("test%%d", i), func (%s *testing.T) {

				})
			}
		}
	`, r.targetPackage.Name()+"_test", testFuncName, testingT)

	fset := token.NewFileSet()
	testFileName := "test.go"
	f, err := parser.ParseFile(fset, testFileName, testTemp, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate AST for test module")
	}
	astutil.AddImport(fset, f, "fmt")
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
	for _, runResult := range r.RunResults {
		// Add symbol values
		symbolValues := runResult.symbolValues
		tc := &ast.CompositeLit{}
		for i, value := range symbolValues {
			ty := r.SymbolTypes[i]
			tc.Elts = append(tc.Elts, value2ASTExpr(value, ty))
		}

		// Add oracle values
		returnValues := runResult.returnValues
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
	testRunCallExpr := testRangeStmtBody.List[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	testRunFuncExpr := testRunCallExpr.Args[1].(*ast.FuncLit)
	testRunFuncExpr.Body.List = runnerFunc.Body.List
	r.insertAuxiliaryFuncs(f)

	return f, nil
}

func (r *ExecuteResult) insertAuxiliaryFuncs(f *ast.File) {
	insertFuncs := make(map[string]*ast.FuncDecl)

	for i, ty := range r.SymbolTypes {
		pointerTy, ok := ty.(*types.Pointer)
		if !ok {
			continue
		}
		elemTy, ok := pointerTy.Elem().(*types.Basic)
		if !ok {
			continue
		}
		name := elemTy.Name() + "ptr"
		if _, ok := insertFuncs[name]; ok {
			continue
		}
		for _, rr := range r.RunResults {
			v := rr.symbolValues[i]
			if v != nil {
				insertFuncs[name] = getAuxiliaryPtrFunc(name, elemTy)
				break
			}
		}
	}

	insertPos := len(f.Decls)
	for i, decl := range f.Decls {
		if _, ok := decl.(*ast.FuncDecl); ok {
			insertPos = i
			break
		}
	}

	newDecls := []ast.Decl{}
	for _, decl := range insertFuncs {
		newDecls = append(newDecls, decl)
	}
	f.Decls = append(f.Decls[:insertPos], append(newDecls, f.Decls[insertPos:]...)...)
}

func getAuxiliaryPtrFunc(name string, ty *types.Basic) *ast.FuncDecl {
	return &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					&ast.Field{
						Names: []*ast.Ident{ast.NewIdent("x")},
						Type:  ast.NewIdent(ty.Name()),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{&ast.Field{Type: &ast.StarExpr{X: ast.NewIdent(ty.Name())}}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.UnaryExpr{
							Op: token.AND,
							X:  ast.NewIdent("x"),
						},
					},
				},
			},
		},
	}
}

func (r *ExecuteResult) rewriteSymbols(runnerFunc *ast.FuncDecl) ([]string, []string, error) {
	symbolType := r.congoSymbolPackage.Scope().Lookup("SymbolType").Type()
	retValType := r.congoSymbolPackage.Scope().Lookup("RetValType").Type()
	var err error

	symbolNames := make([]string, len(r.SymbolTypes))
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
		ty := r.runnerTypesInfo.TypeOf(indexExpr)
		if !(ty == symbolType || ty == retValType) {
			return true
		}
		indexTV, ok := r.runnerTypesInfo.Types[indexExpr.Index]
		if !(ok && indexTV.Value.Kind() == constant.Int) {
			err = errors.New("indexing symbols should be constant")
			return false
		}
		i, _ := constant.Int64Val(indexTV.Value)
		var name string
		switch ty {
		case symbolType:
			if callExpr, ok := c.Parent().(*ast.CallExpr); ok {
				sig := r.runnerTypesInfo.TypeOf(callExpr.Fun).(*types.Signature)
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
		funcType := r.runnerTypesInfo.TypeOf(callExpr.Fun)
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
