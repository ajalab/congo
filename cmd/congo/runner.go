package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/ajalab/congo/cmd/congo/interp"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type Config struct {
	PackageName string
	FuncName    string
}

type Program struct {
	PackageName   string
	FuncName      string
	packageRunner *ssa.Package
	mainFunc      *ssa.Function
	targetFunc    *ssa.Function
	symbols       []types.Type
}

func init() {
	log.SetFlags(log.Llongfile)
}

const packageRunnerPath = "congomain"
const packageCongoPath = "github.com/ajalab/congo"

func (c *Config) Open() (*Program, error) {
	runnerFile, err := generateRunnerFile(c.PackageName, c.FuncName)
	if err != nil {
		return nil, err
	}

	// Load and type-check
	var loaderConf loader.Config
	loaderConf.CreateFromFiles(packageRunnerPath, runnerFile)
	loaderConf.Import(packageCongoPath)
	loaderProg, err := loaderConf.Load()
	if err != nil {
		return nil, err
	}

	// Convert to SSA form
	ssaProg := ssautil.CreateProgram(loaderProg, ssa.BuilderMode(0))
	ssaProg.Build()

	// Find SSA package of the runner
	var packageRunner, packageCongo, packageTarget *ssa.Package
	for _, info := range loaderProg.AllPackages {
		switch info.Pkg.Path() {
		case packageRunnerPath:
			packageRunner = ssaProg.Package(info.Pkg)
		case packageCongoPath:
			packageCongo = ssaProg.Package(info.Pkg)
		case c.PackageName:
			packageTarget = ssaProg.Package(info.Pkg)
		}
	}

	if packageRunner == nil || packageCongo == nil || packageTarget == nil {
		// unreachable
		return nil, fmt.Errorf("runner package or %s does not exist", packageCongoPath)
	}

	// Find references to congo.Symbol
	symbolType := packageCongo.Members["SymbolType"].Type()
	mainFunc := packageRunner.Func("main")
	symbolSubstTable := make(map[uint64]struct {
		int
		types.Type
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
				if subst.Type != ty {
					return nil, fmt.Errorf("Symbol[%d] is used as multiple types", i)
				}
				indexAddrInstr.Index = ssa.NewConst(constant.MakeUint64(uint64(subst.int)), index.Type())
			} else {
				newi := len(symbolSubstTable)
				indexAddrInstr.Index = ssa.NewConst(constant.MakeUint64(uint64(newi)), index.Type())
				symbolSubstTable[i] = struct {
					int
					types.Type
				}{newi, ty}
			}
		}
	}
	symbols := make([]types.Type, len(symbolSubstTable))
	for _, subst := range symbolSubstTable {
		symbols[subst.int] = subst.Type
	}

	return &Program{
		PackageName:   c.PackageName,
		FuncName:      c.FuncName,
		packageRunner: packageRunner,
		mainFunc:      mainFunc,
		targetFunc:    packageTarget.Func(c.FuncName),
		symbols:       symbols,
	}, nil
}

func generateRunnerFile(packageName, funcName string) (*ast.File, error) {
	packageSplit := strings.Split(packageName, "/")
	packageIdent := packageSplit[len(packageSplit)-1]

	// Get argument types of the function
	argTypes, err := getArgTypes(packageName, funcName)
	if err != nil {
		return nil, err
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
							Value: fmt.Sprintf("\"%s\"", packageCongoPath),
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
	var loaderConf loader.Config

	loaderConf.Import(packageName)
	loaderProg, err := loaderConf.Load()
	if err != nil {
		return nil, err
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
		var typeExpr ast.Expr
		switch ty := ty.(type) {
		case *types.Basic:
			typeExpr = ast.NewIdent(ty.Name())
		case *types.Named:
			typeExpr = &ast.SelectorExpr{
				X:   ast.NewIdent(ty.Obj().Pkg().Name()),
				Sel: ast.NewIdent(ty.Obj().Id()),
			}
		}

		arg := &ast.TypeAssertExpr{
			X: &ast.IndexExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("congo"),
					Sel: ast.NewIdent("Symbols"),
				},
				Index: &ast.BasicLit{
					Kind:  token.INT,
					Value: strconv.Itoa(i),
				},
			},
			Type: typeExpr,
		}
		args = append(args, arg)
	}

	return args
}

func (program *Program) Run() error {
	n := len(program.symbols)
	symbolValues := make([]interp.SymbolicValue, n)
	for i := 0; i < n; i++ {
		symbolValues[i] = interp.SymbolicValue{Value: int32(0), Type: program.symbols[i]}
	}

	program.mainFunc.WriteTo(os.Stdout)

	mode := interp.DisableRecover // interp.EnableTracing
	interp.Interpret(
		program.packageRunner,
		program.targetFunc,
		symbolValues,
		mode,
		&types.StdSizes{WordSize: 8, MaxAlign: 8},
		"",
		[]string{})
	return nil
}
