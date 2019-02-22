package congo

import (
	"fmt"
	"testing"
)

func strSetEqual(xs, ys []string) bool {
	if len(xs) != len(ys) {
		return false
	}

	m := make(map[string]uint)
	for _, x := range xs {
		m[x] = m[x] + 1
	}

	for _, y := range ys {
		c, ok := m[y]
		if !ok {
			return false
		}
		if c == 1 {
			delete(m, y)
		} else {
			m[y] = c - 1
		}
	}
	return len(m) == 0
}

func TestLoadTargetPackage(t *testing.T) {
	packagePaths := []string{
		"github.com/ajalab/congo/testdata",
		"testdata/pointer.go",
	}

	for i, packagePath := range packagePaths {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, err := loadTargetPackage(packagePath)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoadTargetFuncs(t *testing.T) {
	zeroExecuteOption := &ExecuteOption{}
	myExecuteOption := &ExecuteOption{MaxExec: 100}
	fooExecuteOption := &ExecuteOption{MaxExec: 10, MinCoverage: 0.75}
	barExecuteOption := &ExecuteOption{MaxExec: 50, MinCoverage: defaultExecuteOption.MinCoverage}
	tcs := []struct {
		packagePath string
		funcNames   []string
		argEO       *ExecuteOption
		ans         map[string]*ExecuteOption
	}{
		{
			"testdata/load/foo.go",
			nil,
			zeroExecuteOption,
			map[string]*ExecuteOption{"AnnotatedFoo": fooExecuteOption},
		},
		{
			"testdata/load/foo.go",
			nil,
			myExecuteOption,
			map[string]*ExecuteOption{
				"AnnotatedFoo": {
					MaxExec: myExecuteOption.MaxExec, MinCoverage: fooExecuteOption.MinCoverage,
				},
			},
		},
		{
			"testdata/load/foo.go",
			[]string{"AnnotatedFoo", "NonAnnotatedFoo"},
			zeroExecuteOption,
			map[string]*ExecuteOption{
				"AnnotatedFoo":    fooExecuteOption,
				"NonAnnotatedFoo": defaultExecuteOption,
			},
		},
		{
			"testdata/load/foo.go",
			[]string{"AnnotatedFoo", "NonAnnotatedFoo"},
			myExecuteOption,
			map[string]*ExecuteOption{
				"AnnotatedFoo": {
					MaxExec: myExecuteOption.MaxExec, MinCoverage: fooExecuteOption.MinCoverage,
				},
				"NonAnnotatedFoo": {
					MaxExec: myExecuteOption.MaxExec, MinCoverage: defaultExecuteOption.MinCoverage,
				},
			},
		},
		{
			"github.com/ajalab/congo/testdata/load",
			nil,
			zeroExecuteOption,
			map[string]*ExecuteOption{
				"AnnotatedFoo": fooExecuteOption,
				"AnnotatedBar": barExecuteOption,
			},
		},
		{
			"github.com/ajalab/congo/testdata/load",
			[]string{"AnnotatedBar"},
			zeroExecuteOption,
			map[string]*ExecuteOption{
				"AnnotatedBar": barExecuteOption,
			},
		},
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			targetPackage, err := loadTargetPackage(tc.packagePath)
			if err != nil {
				t.Fatalf("%s: %v", tc.packagePath, err)
			}
			targets, err := loadTargetFuncs(
				tc.packagePath,
				targetPackage,
				tc.funcNames,
				tc.argEO,
			)
			if err != nil {
				t.Fatalf("%s: %v", tc.packagePath, err)
			}

			if len(targets) != len(tc.ans) {
				t.Fatalf("expected: %v, actual: %v", tc.ans, targets)
			}

			for k, a := range targets {
				e, ok := tc.ans[k]
				if !ok {
					t.Fatalf("function \"%s\" is an unexpected target", k)
				}
				if a.MaxExec != e.MaxExec || a.MinCoverage != e.MinCoverage {
					t.Errorf("execute options are wrong for function %s: expected %+v, actual %+v", k, e, a.ExecuteOption)
				}
			}
		})
	}
}
