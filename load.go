package congo

import (
	"fmt"
	"go/constant"
	"go/format"
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

	format.Node(runnerTmpFile, token.NewFileSet(), runnerFile)

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

	ssaProg, ssaPkgs := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	ssaProg.Build()

	// Find SSA package of the runner
	var runnerPackage, congoSymbolPackage, targetPackage *ssa.Package
	for i, ssaPkg := range ssaPkgs {
		fmt.Printf("%d: %s\n", i, ssaPkg)
		switch ssaPkg.Pkg.Path() {
		case packageCongoSymbolPath:
			congoSymbolPackage = ssaPkg
		case packageName:
			targetPackage = ssaPkg
		default:
			if ssaPkg.Pkg.Name() == "main" {
				runnerPackage = ssaPkg
			}
		}
	}

	if runnerPackage == nil || congoSymbolPackage == nil || targetPackage == nil {
		// unreachable
		return nil, errors.Errorf("runner package or %s does not exist", packageCongoSymbolPath)
	}

	// Find references to congo.Symbol
	symbolType := congoSymbolPackage.Members["SymbolType"].Type()
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
		runnerPackageInfo:  nil, // loaderProg.Package(packageRunnerPath),
		runnerPackage:      runnerPackage,
		targetPackage:      targetPackage,
		congoSymbolPackage: congoSymbolPackage,
		targetFunc:         targetPackage.Func(funcName),
		symbols:            symbols,
	}, nil
}
