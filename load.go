package congo

import (
	"go/ast"
	"go/constant"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"

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

// parseAnnotationDirective parses directives from s, which is in the form of "congo:<key>[ <value>]".
// This function requires that the leading and trailing white space
// in s is trimmed beforehand.
func parseAnnotationDirective(s, key string) (string, bool) {
	prefix := "congo:"
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	prefixTrimmed := strings.TrimPrefix(s, prefix)

	var read []rune
	for _, r := range prefixTrimmed {
		if unicode.IsSpace(r) {
			break
		}
		read = append(read, r)
	}

	readKey := string(read)
	if readKey != key {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(prefixTrimmed, readKey))

	return value, true
}

func parseAnnotation(text string, eo *ExecuteOption) (bool, error) {
	const annoTagKey = "key"

	if text[:2] != "//" {
		return false, nil
	}
	text = strings.TrimSpace(text[2:])

	eoTy := reflect.TypeOf(*eo)
	for i := 0; i < eoTy.NumField(); i++ {
		f := eoTy.Field(i)
		key := f.Tag.Get(annoTagKey)
		if value, ok := parseAnnotationDirective(text, key); ok {
			switch f.Type.Kind() {
			case reflect.Uint:
				iv, err := strconv.Atoi(value)
				if err != nil {
					return false, err
				}
				reflect.ValueOf(eo).Elem().Field(i).SetUint(uint64(iv))
			case reflect.Float64:
				fv, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return false, err
				}
				reflect.ValueOf(eo).Elem().Field(i).SetFloat(fv)
			default:
				return false, errors.Errorf("unsupported option tyupe: %s", f.Type)
			}
			return true, nil
		}
	}
	return false, nil
}

func getExecuteOption(funcDecl *ast.FuncDecl, cgroups []*ast.CommentGroup) (*ExecuteOption, error) {
	eo := &ExecuteOption{}
	isTarget := false
	for _, cgroup := range cgroups {
		for _, comment := range cgroup.List {
			text := comment.Text
			parsed, err := parseAnnotation(text, eo)
			if err != nil {
				return eo, errors.Wrapf(err, `failed to parse annotation "%s" for %s`, text, funcDecl.Name.Name)
			}
			isTarget = isTarget || parsed
		}
	}
	if isTarget {
		return eo, nil
	}
	return nil, nil
}

func loadTargetFuncs(
	targetPackagePath string,
	targetPackage *packages.Package,
	funcNames []string,
	argEO *ExecuteOption,
) (map[string]*Target, error) {
	// cases:
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
		return nil, errors.Errorf("no files could be loaded for package %s", targetPackagePath)
	}

	cmaps := make([]ast.CommentMap, len(fs))
	for i, f := range fs {
		cmaps[i] = ast.NewCommentMap(targetPackage.Fset, f, f.Comments)
	}

	targets := make(map[string]*Target)
	if len(funcNames) > 0 {
		// case 2 or 4
	FUNC:
		for _, name := range funcNames {
			for j, f := range fs {
				obj := f.Scope.Lookup(name)
				if obj == nil {
					continue
				}
				if funcDecl, ok := obj.Decl.(*ast.FuncDecl); ok {
					eo, err := getExecuteOption(funcDecl, cmaps[j][funcDecl])
					if err != nil {
						return nil, errors.Wrapf(err, "failed to parse annotations for function %s", funcDecl.Name)
					}
					if eo == nil {
						eo = &ExecuteOption{}
					}
					eo.Fill(argEO, true).Fill(defaultExecuteOption, false)
					target := &Target{name: name, ExecuteOption: eo}
					targets[name] = target
					continue FUNC
				}
			}
			return nil, errors.Errorf("function %s does not exist in %s", name, targetPackagePath)
		}
		return targets, nil
	}
	// case 1 or 3
	for i, f := range fs {
		for _, decl := range f.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				name := funcDecl.Name.String()
				eo, err := getExecuteOption(funcDecl, cmaps[i][funcDecl])
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse annotations for function %s", funcDecl.Name)
				}
				if eo == nil {
					continue
				}
				eo.Fill(argEO, true).Fill(defaultExecuteOption, false)
				target := &Target{name: name, ExecuteOption: eo}
				targets[name] = target
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

	targets, err := loadTargetFuncs(targetPackagePath, targetPackage, config.FuncNames, &config.ExecuteOption)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load package %s", targetPackagePath)
	}

	// Generate a runner file if config.Runner is nil.
	runnerPackageFPath := config.Runner
	if runnerPackageFPath == "" {
		runnerPackageFPath, err = generateRunner(targetPackage, targets)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate a runner")
		}
		defer os.Remove(runnerPackageFPath)
	} else {
		return nil, errors.New("user-specified runner is not supported yet")
	}

	// IPath represents an import path.
	targetPackageIPath := targetPackage.PkgPath
	return load(targets, targetPackageIPath, runnerPackageFPath)
}

func load(targets map[string]*Target, targetPackageIPath, runnerPackagePath string) (*Congo, error) {
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
	symbolType := congoSymbolPackageSSA.Members["SymbolType"].Type()

	for _, target := range targets {
		// Find references to congo.Symbol
		mainFunc := runnerPackageSSA.Func(target.runnerName)
		symbolSubstTable := make(map[uint64]struct {
			i int
			v ssa.Value
		})
		for _, block := range mainFunc.Blocks {
			for _, instr := range block.Instrs {
				// expression symbol.Symbols[i].(XXX) is considered as a symbol.
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

		target.f = targetPackageSSA.Func(target.name)
		target.symbols = symbols
	}

	program := &Program{
		runnerFile:         runnerPackage.Syntax[0],
		runnerTypesInfo:    runnerPackage.TypesInfo,
		runnerPackage:      runnerPackageSSA,
		targetPackage:      targetPackageSSA,
		congoSymbolPackage: congoSymbolPackageSSA,
	}

	return &Congo{
		program: program,
		targets: targets,
	}, nil
}
