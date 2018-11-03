package congo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

func generateRunner(packageName, funcName string) (*ast.File, error) {
	packageSplit := strings.Split(packageName, "/")
	packageIdent := packageSplit[len(packageSplit)-1]

	// Get argument types of the function
	sig, err := getTargetFuncSig(packageName, funcName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get argument types of the function")
	}

	results := sig.Results()
	retValsLen := results.Len()
	assertRetVals := make(map[*types.Var]struct{})
	for i := 0; i < retValsLen; i++ {
		v := results.At(i)
		if _, ok := v.Type().(*types.Basic); ok {
			assertRetVals[v] = struct{}{}
		}
	}

	args := generateSymbolicArgs(sig)
	funcCallExpr := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(packageIdent),
			Sel: ast.NewIdent(funcName),
		},
		Args: args,
	}
	var funcCallStmt ast.Stmt
	var runnerFuncBody *ast.BlockStmt
	if len(assertRetVals) == 0 {
		funcCallStmt = &ast.ExprStmt{X: funcCallExpr}
		runnerFuncBody = &ast.BlockStmt{List: []ast.Stmt{funcCallStmt}}
	} else {
		lhs := make([]ast.Expr, retValsLen)
		var assertCond ast.Expr
		for i := 0; i < retValsLen; i++ {
			v := results.At(i)
			if _, ok := assertRetVals[v]; ok {
				name := v.Name()
				if name == "" {
					name = fmt.Sprintf("actual%d", i)
				}
				lhs[i] = ast.NewIdent(name)

				cond := &ast.BinaryExpr{
					X: ast.NewIdent(name),
					Y: &ast.TypeAssertExpr{
						X: &ast.IndexExpr{
							X: &ast.SelectorExpr{
								X:   ast.NewIdent("symbol"),
								Sel: ast.NewIdent("RetVals"),
							},
							Index: &ast.BasicLit{
								Kind:  token.INT,
								Value: strconv.Itoa(i),
							},
						},
						Type: type2ASTExpr(v.Type()),
					},
					Op: token.EQL,
				}

				if assertCond == nil {
					assertCond = cond
				} else {
					assertCond = &ast.BinaryExpr{
						X:  assertCond,
						Y:  cond,
						Op: token.LAND,
					}
				}
			}
		}
		funcCallStmt = &ast.AssignStmt{
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

	runnerFuncDecl := &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{},
		Body: runnerFuncBody,
	}

	scope := ast.NewScope(nil)
	runnerFuncDeclObj := ast.NewObj(ast.Fun, "main")
	runnerFuncDeclObj.Decl = runnerFuncDecl
	scope.Insert(runnerFuncDeclObj)

	return &ast.File{
		Scope: scope,
		Name:  ast.NewIdent(packageRunnerPath),
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok: token.IMPORT,
				Specs: []ast.Spec{
					&ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf("\"%s\"", packageName),
						},
					},
				},
			},
			&ast.GenDecl{
				Tok: token.IMPORT,
				Specs: []ast.Spec{
					&ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf("\"%s\"", packageCongoSymbolPath),
						},
					},
				},
			},
			runnerFuncDecl,
		},
	}, nil
}

func getTargetFuncSig(packageName string, funcName string) (*types.Signature, error) {
	conf := &packages.Config{
		Mode: packages.LoadTypes,
	}
	pkgs, err := packages.Load(conf, packageName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the target package")
	}
	targetFunc := pkgs[0].Types.Scope().Lookup(funcName)
	if targetFunc == nil {
		return nil, errors.Errorf("function %s does not exist in package %s", funcName, packageName)
	}
	targetFuncType := targetFunc.Type()
	sig, ok := targetFuncType.(*types.Signature)
	if !ok {
		// unreachable
		return nil, errors.Errorf("%s is not a function", funcName)
	}

	return sig, nil
}

func generateSymbolicArgs(sig *types.Signature) []ast.Expr {
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
