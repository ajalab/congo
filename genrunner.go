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
	var assertRetVals []*types.Var
	for i := 0; i < retValsLen; i++ {
		v := results.At(i)
		if _, ok := v.Type().(*types.Basic); ok {
			assertRetVals = append(assertRetVals, v)
		}
	}
	fmt.Printf("assertletvals: %+v\n", assertRetVals)

	args := generateSymbolicArgs(sig)
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
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.ExprStmt{
							X: &ast.CallExpr{
								Fun: &ast.SelectorExpr{
									X:   ast.NewIdent(packageIdent),
									Sel: ast.NewIdent(funcName),
								},
								Args: args,
							},
						},
					},
				},
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
