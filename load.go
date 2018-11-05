package congo

import (
	"go/constant"
	"go/token"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/tools/go/packages"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/pkg/errors"
)

func init() {
	log.SetFlags(log.Llongfile)
}

const packageCongoSymbolPath = "github.com/ajalab/congo/symbol"

// Load loads the target program
func Load(packageName string, funcName string) (*Program, error) {
	runnerFile, err := generateRunner(packageName, funcName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate runner AST file")
	}
	runnerTmpFile, err := ioutil.TempFile("", "*.go")
	if err != nil {
		return nil, err
	}
	defer os.Remove(runnerTmpFile.Name())

	if err := runnerTmpFile.Close(); err != nil {
		return nil, err
	}

	config := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}
	pkgs, err := packages.Load(config, "github.com/ajalab/congo/symbol", packageName, "file="+runnerTmpFile.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to load packages")
	}

	// Error-check and get indices for each package
	var runnerPackage, congoSymbolPackage, targetPackage int
	for i, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			err = errors.Errorf("failed to load package %s: %v", pkg.PkgPath, pkg.Errors)
			break
		}
		switch pkg.PkgPath {
		case packageCongoSymbolPath:
			congoSymbolPackage = i
		case packageName:
			targetPackage = i
		default:
			if pkg.Name == "main" {
				runnerPackage = i
			} else {
				err = errors.Errorf("a non-relevant package was found: %s", pkg.PkgPath)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	ssaProg, ssaPkgs := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	ssaProg.Build()

	// Find references to congo.Symbol
	symbolType := ssaPkgs[congoSymbolPackage].Members["SymbolType"].Type()
	mainFunc := ssaPkgs[runnerPackage].Func("main")
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
				return nil, errors.Errorf("Illegal use of Symbol")
			}
			indexAddrInstr, ok := unopInstr.X.(*ssa.IndexAddr)
			if !ok {
				return nil, errors.Errorf("Symbol must be used with the index operator")
			}
			index, ok := indexAddrInstr.Index.(*ssa.Const)
			if !ok {
				return nil, errors.Errorf("Symbol must be indexed with a constant value")
			}

			i := index.Uint64()
			if subst, ok := symbolSubstTable[i]; ok {
				if subst.v.Type() != ty {
					return nil, errors.Errorf("Symbol[%d] is used as multiple types", i)
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
		runnerFile:         runnerFile,
		runnerPackage:      ssaPkgs[runnerPackage],
		targetPackage:      ssaPkgs[targetPackage],
		congoSymbolPackage: ssaPkgs[congoSymbolPackage],
		targetFunc:         ssaPkgs[targetPackage].Func(funcName),
		symbols:            symbols,
	}, nil
}
