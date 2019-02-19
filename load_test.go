package congo

import (
	"fmt"
	"testing"
)

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
	tcs := []struct {
		packagePath string
		funcNames   []string
		ans         []string
	}{
		{
			"testdata/load/foo.go",
			nil,
			[]string{"AnnotatedFoo", "NonAnnotatedFoo"},
		},
		{
			"testdata/load/foo.go",
			[]string{"AnnotatedFoo"},
			[]string{"AnnotatedFoo"},
		},
		{
			"github.com/ajalab/congo/testdata/load",
			nil,
			[]string{"AnnotatedFoo", "NonAnnotatedFoo", "AnnotatedBar", "NonAnnotatedBar"},
		},
		{
			"github.com/ajalab/congo/testdata/load",
			[]string{"AnnotatedFoo"},
			[]string{"AnnotatedFoo"},
		},
	}

	eo := &ExecuteOption{}
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
				eo,
			)
			if err != nil {
				t.Fatalf("%s: %v", tc.packagePath, err)
			}

			set := make(map[string]struct{})
			for _, a := range tc.ans {
				set[a] = struct{}{}
			}

			actual := make([]string, len(targets))
			for i, target := range targets {
				if _, ok := set[target.name]; !ok {
					t.Fatalf("expected: %v, actual: %v", tc.ans, actual)
				}
				actual[i] = target.name
			}
			if len(set) != len(actual) {
			}
		})
	}
}
