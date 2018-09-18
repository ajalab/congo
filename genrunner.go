package congo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

func generateRunnerFile(packageName, funcName string) (*ast.File, error) {
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

	return &ast.File{
		Name: ast.NewIdent(packageRunnerPath),
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
			&ast.FuncDecl{
				Name: ast.NewIdent("main"),
				Type: &ast.FuncType{},
				Body: runnerFuncBody,
			},
		},
	}, nil
}

func getTargetFuncSig(packageName string, funcName string) (*types.Signature, error) {
	// TODO(ajalab):
	// We only need to load the target package but not its dependencies.
	loaderConf := loader.Config{
		TypeCheckFuncBodies: func(_ string) bool { return false },
	}

	loaderConf.Import(packageName)
	loaderProg, err := loaderConf.Load()
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to load package %s", packageName))
	}
	pkg := loaderProg.Package(packageName).Pkg
	function := pkg.Scope().Lookup(funcName)
	if function == nil {
		return nil, fmt.Errorf("function %s does not exist in package %s", funcName, packageName)
	}
	funcType := function.Type()
	sig, ok := funcType.(*types.Signature)
	if !ok {
		// unreachable
		return nil, fmt.Errorf("%s is not a function", funcName)
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
