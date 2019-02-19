package congo

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"io/ioutil"
	"strconv"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

func generateRunner(targetPackage *packages.Package, targets []*Target) (string, error) {
	runnerFile, err := generateRunnerAST(targetPackage, targets)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate runner AST file")
	}
	runnerTmpFile, err := ioutil.TempFile("", "*.go")
	if err != nil {
		return "", err
	}
	runnerPackageFPath := runnerTmpFile.Name()

	format.Node(runnerTmpFile, token.NewFileSet(), runnerFile)
	if err := runnerTmpFile.Close(); err != nil {
		return "", err
	}

	return runnerPackageFPath, nil
}

// generateRunner generates the AST of a test runner.
// The runner calls the target function declared in targetPackage.
func generateRunnerAST(targetPackage *packages.Package, targets []*Target) (*ast.File, error) {
	scope := ast.NewScope(nil)
	runnerFuncDecls := make([]*ast.FuncDecl, len(targets))
	for i, target := range targets {
		runnerFuncDecl, err := generateRunnerFuncAST(targetPackage, target.name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate a runner function declaration AST for %s", target.name)
		}
		runnerFuncDecls[i] = runnerFuncDecl

		runnerFuncName := runnerFuncDecl.Name.Name
		// Ties the runner function to the scope.
		runnerFuncDeclObj := ast.NewObj(ast.Fun, runnerFuncName)
		runnerFuncDeclObj.Decl = runnerFuncDecl
		scope.Insert(runnerFuncDeclObj)

		// Update the runnerName field in target.
		target.runnerName = runnerFuncName
	}

	// var __congoRunner func()
	runnerVarDecl := &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent(runnerFuncNamePrefix)},
				Type:  &ast.FuncType{},
			},
		},
	}

	// func main() {
	//     __congoRunner()
	// }
	mainFuncDecl := &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: ast.NewIdent(runnerFuncNamePrefix),
					},
				},
			},
		},
	}
	mainFuncDeclObj := ast.NewObj(ast.Fun, mainFuncDecl.Name.Name)
	mainFuncDeclObj.Decl = mainFuncDecl
	scope.Insert(mainFuncDeclObj)

	// We do not use parenthesized import declaration
	// since it needs the valid Lparen position.
	decls := []ast.Decl{
		generateImportDeclAST("", targetPackage.PkgPath),
		generateImportDeclAST("", congoSymbolPackagePath),
		// runtime package is required to run by interp
		generateImportDeclAST("_", "runtime"),
		runnerVarDecl,
		mainFuncDecl,
	}
	for _, decl := range runnerFuncDecls {
		decls = append(decls, decl)
	}

	return &ast.File{
		Scope: scope,
		Name:  ast.NewIdent("main"),
		Decls: decls,
	}, nil
}

// Get the signature of the function that belongs to pkg
func getTargetFuncSig(pkg *packages.Package, funcName string) (*types.Signature, error) {
	targetFunc := pkg.Types.Scope().Lookup(funcName)
	if targetFunc == nil {
		return nil, errors.Errorf("function %s does not exist in package %s", funcName, pkg)
	}
	targetFuncType := targetFunc.Type()
	sig, ok := targetFuncType.(*types.Signature)
	if !ok {
		return nil, errors.Errorf("%s is not a function", funcName)
	}

	return sig, nil
}

const runnerFuncNamePrefix = "__congoRunner"

func generateRunnerFuncAST(targetPackage *packages.Package, funcName string) (*ast.FuncDecl, error) {
	// Get the signature of the target function
	sig, err := getTargetFuncSig(targetPackage, funcName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get the signature of %s", funcName)
	}

	// Generate AST of the function call to the target function
	// targetPackage.targetFunc(arg0, arg1, ...)
	args := generateSymbolASTs(sig)
	funcCallExpr := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(targetPackage.Name),
			Sel: ast.NewIdent(funcName),
		},
		Args: args,
	}

	// Generate
	// - LHS of the assignStmt that stores the results of the target function call
	// - assert conditions
	var lhs []ast.Expr
	var assertCond ast.Expr
	results := sig.Results()
	retValsLen := results.Len()
	assertResultsLen := 0
	for i := 0; i < retValsLen; i++ {
		v := results.At(i)
		if _, ok := v.Type().(*types.Basic); ok {
			name := v.Name()
			if name == "" {
				name = fmt.Sprintf("actual%d", assertResultsLen)
			}
			// actualN == symbol.RetVals[N].(type of actualN)
			cond := &ast.BinaryExpr{
				Op: token.EQL,
				X:  ast.NewIdent(name),
				Y: &ast.TypeAssertExpr{
					X: &ast.IndexExpr{
						X: &ast.SelectorExpr{
							X: ast.NewIdent("symbol"),
							// TODO(ajalab): Avoid hard coding
							Sel: ast.NewIdent("RetVals"),
						},
						Index: &ast.BasicLit{
							Kind:  token.INT,
							Value: strconv.Itoa(assertResultsLen),
						},
					},
					Type: type2ASTExpr(v.Type()),
				},
			}
			// Conjunction
			if assertCond == nil {
				assertCond = cond
			} else {
				assertCond = &ast.BinaryExpr{
					Op: token.LAND,
					X:  assertCond,
					Y:  cond,
				}
			}
			lhs = append(lhs, ast.NewIdent(name))
			assertResultsLen++
		} else {
			lhs = append(lhs, ast.NewIdent("_"))
		}
	}

	// Generate the function body of a runner
	var runnerFuncBody *ast.BlockStmt
	if assertResultsLen == 0 {
		// No assertions
		// {
		//      targetPackage.targetFunc(arg0, arg1, ...)
		// }
		funcCallStmt := &ast.ExprStmt{X: funcCallExpr}
		runnerFuncBody = &ast.BlockStmt{List: []ast.Stmt{funcCallStmt}}
	} else {
		// Assertions exist
		// {
		//      actual0, actual1, ... := targetPackage.targetFunc(arg0, arg1, ...)
		//      symbol.TestAssert(
		//          actual0 == symbol.RetVals[0].(type of actual0) &&
		//          actual1 == symbol.RetVals[1].(type of actual1) &&
		//          ...
		//      )
		// }
		funcCallStmt := &ast.AssignStmt{
			Tok: token.DEFINE,
			Lhs: lhs,
			Rhs: []ast.Expr{funcCallExpr},
		}
		assertStmt := &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("symbol"),
					Sel: ast.NewIdent("TestAssert"),
				},
				Args: []ast.Expr{assertCond},
			},
		}
		runnerFuncBody = &ast.BlockStmt{List: []ast.Stmt{funcCallStmt, assertStmt}}
	}

	// func __congoRunnerXXX() {
	//     (runnerFuncBody)
	// }
	runnerFuncDecl := &ast.FuncDecl{
		Name: ast.NewIdent(runnerFuncNamePrefix + funcName),
		Type: &ast.FuncType{},
		Body: runnerFuncBody,
	}

	return runnerFuncDecl, nil
}

func generateSymbolASTs(sig *types.Signature) []ast.Expr {
	argTypes := sig.Params()
	argLen := argTypes.Len()
	var args []ast.Expr
	for i := 0; i < argLen; i++ {
		ty := argTypes.At(i).Type()
		args = append(args, &ast.TypeAssertExpr{
			X: &ast.IndexExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("symbol"),
					Sel: ast.NewIdent("Symbols"),
				},
				Index: &ast.BasicLit{
					Kind:  token.INT,
					Value: strconv.Itoa(i),
				},
			},
			Type: type2ASTExpr(ty),
		})
	}

	return args
}

func generateImportDeclAST(name, path string) *ast.GenDecl {
	var alias *ast.Ident
	if name != "" {
		alias = ast.NewIdent(name)
	}
	return &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: fmt.Sprintf("\"%s\"", path),
				},
				Name: alias,
			},
		},
	}
}
