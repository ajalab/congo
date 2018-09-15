package congo

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"strconv"
	"strings"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/pkg/errors"
)

type Config struct {
	PackageName string
	FuncName    string
}

func init() {
	log.SetFlags(log.Llongfile)
}

const packageCongoSymbolPath = "github.com/ajalab/congo/symbol"
const packageRunnerPath = "congomain"

func (c *Config) Open() (*Program, error) {
	runnerFile, err := generateRunnerFile(c.PackageName, c.FuncName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate runner AST file")
	}

	// format.Node(os.Stdout, token.NewFileSet(), runnerFile)

	// Load and type-check
	var loaderConf loader.Config
	loaderConf.CreateFromFiles(packageRunnerPath, runnerFile)
	loaderConf.Import(packageCongoSymbolPath)
	loaderProg, err := loaderConf.Load()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load packages")
	}

	// Convert to SSA form
	ssaProg := ssautil.CreateProgram(loaderProg, ssa.BuilderMode(0))
	ssaProg.Build()

	// Find SSA package of the runner
	var runnerPackage, packageCongoSymbol, targetPackage *ssa.Package
	for _, info := range loaderProg.AllPackages {
		switch info.Pkg.Path() {
		case packageRunnerPath:
			runnerPackage = ssaProg.Package(info.Pkg)
		case packageCongoSymbolPath:
			packageCongoSymbol = ssaProg.Package(info.Pkg)
		case c.PackageName:
			targetPackage = ssaProg.Package(info.Pkg)
		}
	}

	if runnerPackage == nil || packageCongoSymbol == nil || targetPackage == nil {
		// unreachable
		return nil, fmt.Errorf("runner package or %s does not exist", packageCongoSymbolPath)
	}

	// Find references to congo.Symbol
	symbolType := packageCongoSymbol.Members["SymbolType"].Type()
	mainFunc := runnerPackage.Func("main")
	symbolSubstTable := make(map[uint64]struct {
		i int
		v ssa.Value
	})
	for _, block := range mainFunc.Blocks {
		for _, instr := range block.Instrs {
			// Check if instr is pointer indirection ( exp.(type) form )
			assertInstr, ok := instr.(*ssa.TypeAssert)
			if !ok || assertInstr.X.Type() != symbolType {
				continue
			}
			ty := assertInstr.AssertedType
			unopInstr, ok := assertInstr.X.(*ssa.UnOp)
			if !ok || unopInstr.Op != token.MUL {
				return nil, fmt.Errorf("Illegal use of Symbol")
			}
			indexAddrInstr, ok := unopInstr.X.(*ssa.IndexAddr)
			if !ok {
				return nil, fmt.Errorf("Symbol must be used with the index operator")
			}
			index, ok := indexAddrInstr.Index.(*ssa.Const)
			if !ok {
				return nil, fmt.Errorf("Symbol must be indexed with a constant value")
			}

			i := index.Uint64()
			if subst, ok := symbolSubstTable[i]; ok {
				if subst.v.Type() != ty {
					return nil, fmt.Errorf("Symbol[%d] is used as multiple types", i)
				}
				indexAddrInstr.Index = ssa.NewConst(constant.MakeUint64(uint64(subst.i)), index.Type())
			} else {
				newi := len(symbolSubstTable)
				indexAddrInstr.Index = ssa.NewConst(constant.MakeUint64(uint64(newi)), index.Type())
				symbolSubstTable[i] = struct {
					i int
					v ssa.Value
				}{newi, assertInstr}
			}
		}
	}
	symbols := make([]ssa.Value, len(symbolSubstTable))
	for _, subst := range symbolSubstTable {
		symbols[subst.i] = subst.v
	}

	return &Program{
		runnerPackage: runnerPackage,
		targetPackage: targetPackage,
		targetFunc:    targetPackage.Func(c.FuncName),
		symbols:       symbols,
	}, nil
}

func generateRunnerFile(packageName, funcName string) (*ast.File, error) {
	packageSplit := strings.Split(packageName, "/")
	packageIdent := packageSplit[len(packageSplit)-1]

	// Get argument types of the function
	argTypes, err := getArgTypes(packageName, funcName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get argument types of the function")
	}
	args := generateSymbolicArgs(argTypes)

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

func getArgTypes(packageName string, funcName string) ([]types.Type, error) {
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

	argTuple := sig.Params()
	argLen := argTuple.Len()
	args := make([]types.Type, argLen)
	for i := 0; i < argLen; i++ {
		args[i] = argTuple.At(i).Type()
	}

	return args, nil
}

func generateSymbolicArgs(argTypes []types.Type) []ast.Expr {
	var args []ast.Expr
	for i, ty := range argTypes {
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
