package congo

import (
	"go/ast"
	"go/constant"
	"go/token"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/pkg/errors"
)

const congoSymbolPackagePath = "github.com/ajalab/congo/symbol"

func loadTargetPackage(targetPackagePath string) (*packages.Package, error) {
	conf := &packages.Config{
		Mode: packages.LoadSyntax,
	}

	query := targetPackagePath
	if isGoFilePath(query) {
		query = "file=" + targetPackagePath
	}
	pkgs, err := packages.Load(conf, query)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load the target package %s", targetPackagePath)
	}
	if len(pkgs) == 0 {
		return nil, errors.Errorf("no packages could be loaded for path %s", targetPackagePath)
	}
	return pkgs[0], nil
}

func isGoFilePath(path string) bool {
	return path[len(path)-3:] == ".go"
}

// Config specifies the (optional) parameters for concolic execution.
// Options are ignored when a field has the zero value.
type Config struct {
	// FuncNames is a list of functions that we generate tests for.
	FuncNames []string
	// Runner is the path to the Go file that calls the target function.
	// Automatically generated if empty string is specified.
	Runner string
	ExecuteOption
}

func parseAnnotation(funcDecl *ast.FuncDecl, cgroups []*ast.CommentGroup) (ExecuteOption, error) {
	return ExecuteOption{}, nil
}

func loadTargetFuncs(targetPackagePath string, targetPackage *packages.Package, config *Config) ([]*Target, error) {
	// 4 cases:
	// 1. targetPackagePath is a file path to a Go file and len(funcNames) is 0.
	// 2. targetPackagePath is a file path to a Go file and len(funcNames) is greater than 0 .
	// 3. targetPackagePath is an import path to the target package and len(funcNames) is 0.
	// 4. targetPackagePath is an import path to the target package and len(funcNames) is greater than 0.

	var fs []*ast.File
	if isGoFilePath(targetPackagePath) {
		targetPackageAbsPath, err := filepath.Abs(targetPackagePath)
		if err != nil {
			return nil, err
		}
		for i, fpath := range targetPackage.CompiledGoFiles {
			if fpath == targetPackageAbsPath {
				fs = append(fs, targetPackage.Syntax[i])
				break
			}
		}
	} else {
		fs = targetPackage.Syntax
	}
	if len(fs) == 0 {
		return nil, errors.New("annotations could not be loaded")
	}

	cmaps := make([]ast.CommentMap, len(fs))
	for i, f := range fs {
		cmaps[i] = ast.NewCommentMap(targetPackage.Fset, f, f.Comments)
	}

	funcNames := config.FuncNames
	if len(funcNames) > 0 {
		targets := make([]*Target, len(funcNames))
		for i, name := range funcNames {
			for j, f := range fs {
				obj := f.Scope.Lookup(name)
				if obj == nil {
					continue
				}
				if funcDecl, ok := obj.Decl.(*ast.FuncDecl); ok {
					eo, err := parseAnnotation(funcDecl, cmaps[j][funcDecl])
					eo.Fill(&config.ExecuteOption, true).Fill(&defaultExecuteOption, false)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to parse annotations for function %s", funcDecl.Name)
					}
					targets[i] = &Target{ExecuteOption: eo}
					break
				}
			}
		}
		return targets, nil
	}
	var targets []*Target
	for i, f := range fs {
		for _, decl := range f.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				eo, err := parseAnnotation(funcDecl, cmaps[i][funcDecl])
				eo.Fill(&config.ExecuteOption, false).Fill(&defaultExecuteOption, false)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse annotations for function %s", funcDecl.Name)
				}
				targets = append(targets, &Target{ExecuteOption: eo})
			}
		}
	}
	return targets, nil
}

// Load loads the target program.
// targetPackagePath is either
// - a file path (e.g., foo/bar.go) to the target package
// - an import path (e.g, github.com/ajalab/congo).
func Load(config *Config, targetPackagePath string) (*Congo, error) {
	if config == nil {
		config = &Config{}
	}

	// (Pre)load the target package to
	// - get a list of target functions from annotations
	// - (optional) generate a test runner
	targetPackage, err := loadTargetPackage(targetPackagePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load package %s", targetPackagePath)
	}

	if _, err := loadTargetFuncs(targetPackagePath, targetPackage, config); err != nil {
		return nil, err
	}

	// Generate a runner file if config.Runner is nil.
	runnerPackageFPath := config.Runner
	if runnerPackageFPath == "" {
		runnerPackageFPath, err = generateRunner(targetPackage, config.FuncNames[0])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate a runner")
		}
		defer os.Remove(runnerPackageFPath)
	}

	// IPath represents an import path.
	targetPackageIPath := targetPackage.PkgPath
	return load(config, targetPackageIPath, runnerPackageFPath)
}

func load(config *Config, targetPackageIPath, runnerPackagePath string) (*Congo, error) {
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
	targetPackage := runnerPackage.Imports[targetPackageIPath]
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
		f:             targetPackageSSA.Func(funcName),
		symbols:       symbols,
		ExecuteOption: config.ExecuteOption,
	}

	return &Congo{
		program: program,
		targets: targets,
	}, nil
}
