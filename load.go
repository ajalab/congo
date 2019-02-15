package congo

import (
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/packages"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/pkg/errors"
)

const congoSymbolPackagePath = "github.com/ajalab/congo/symbol"

// LoadPackage loads a Go package from a package path or a file path.
func LoadPackage(packageName string) (*packages.Package, error) {
	conf := &packages.Config{
		Mode: packages.LoadTypes,
	}
	query := packageName
	if packageName[len(packageName)-3:] == ".go" {
		query = "file=" + packageName
	}
	pkgs, err := packages.Load(conf, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the target package")
	}
	if len(pkgs) == 0 {
		return nil, errors.New("no packages could be loaded")
	}
	return pkgs[0], nil
}

// Config specifies the (optional) parameters for concolic execution.
// Options are ignored when a field has the zero value.
type Config struct {
	// FuncNames is a list of functions that we generate tests for.
	FuncNames []string
	// MaxExec is the maximum number of executions allowed.
	MaxExec uint
	// MinCoverage is the criteria that tells the least coverage ratio to achieve.
	MinCoverage float64
}

// Load loads the target program.
func Load(config *Config, runnerPackagePath, targetPackagePath string) (*Congo, error) {
	if config == nil {
		config = &Config{}
	}

	funcName := config.FuncNames[0]
	pConfig := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}
	pkgs, err := packages.Load(pConfig, runnerPackagePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load packages")
	}
	if len(pkgs) == 0 {
		return nil, errors.New("no packages could be loaded")
	}

	runnerPackageIdx := -1
	for i, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, errors.Errorf("failed to load package %s: %v", pkg.PkgPath, pkg.Errors)
		}
		// It is possible that pkg.IllTyped becomes true but pkg.Errors has no error records.
		if pkg.IllTyped {
			return nil, errors.Errorf("package %s contains type error", pkg.PkgPath)
		}
		if pkg.Name != "runtime" {
			runnerPackageIdx = i
		}
	}
	if runnerPackageIdx < 0 {
		return nil, errors.New("failed to load the runner package")
	}

	runnerPackage := pkgs[runnerPackageIdx]
	targetPackage := runnerPackage.Imports[targetPackagePath]
	congoSymbolPackage := runnerPackage.Imports[congoSymbolPackagePath]

	ssaProg, ssaPkgs := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	for i, ssaPkg := range ssaPkgs {
		if ssaPkg == nil {
			return nil, errors.Errorf("failed to compile package %s into SSA form", pkgs[i])
		}
	}
	ssaProg.Build()

	runnerPackageSSA := ssaPkgs[runnerPackageIdx]
	targetPackageSSA := ssaProg.Package(targetPackage.Types)
	congoSymbolPackageSSA := ssaProg.Package(congoSymbolPackage.Types)

	// Find references to congo.Symbol
	symbolType := congoSymbolPackageSSA.Members["SymbolType"].Type()
	mainFunc := runnerPackageSSA.Func("main")
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

	program := &Program{
		runnerFile:         runnerPackage.Syntax[0],
		runnerTypesInfo:    runnerPackage.TypesInfo,
		runnerPackage:      runnerPackageSSA,
		targetPackage:      targetPackageSSA,
		congoSymbolPackage: congoSymbolPackageSSA,
	}

	targets := make(map[string]*Target)
	targets[funcName] = &Target{
		f:       targetPackageSSA.Func(funcName),
		symbols: symbols,
	}

	return &Congo{
		program: program,
		targets: targets,
	}, nil
}
