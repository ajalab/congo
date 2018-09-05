package congo

import (
	"testing"

	"github.com/ajalab/congo/interp"
)

func TestRun(t *testing.T) {
	testCases := []struct {
		packageName string
		funcName    string
	}{
		{"github.com/ajalab/congo/testdata", "BranchSimple"},
		{"github.com/ajalab/congo/testdata", "BranchAnd"},
		{"github.com/ajalab/congo/testdata", "BranchThreeVars"},
		{"github.com/ajalab/congo/testdata", "BranchTenVars"},
		{"github.com/ajalab/congo/testdata", "BranchStruct"},
	}

	for _, tc := range testCases {
		conf := Config{
			PackageName: tc.packageName,
			FuncName:    tc.funcName,
		}

		prog, err := conf.Open()
		if err != nil {
			t.Fatalf("Config.Open: %v", err)
		}

		n := len(prog.symbols)
		symbolValues := make([]interp.SymbolicValue, n)
		for i := 0; i < n; i++ {
			ty := prog.symbols[i].Type()
			symbolValues[i] = interp.SymbolicValue{
				Value: zero(ty),
				Type:  ty,
			}
		}

		if _, err = prog.Run(symbolValues); err != nil {
			t.Errorf("prog.Run: %v", err)
		}
	}
}

func TestExecute(t *testing.T) {

	testCases := []struct {
		packageName string
		funcName    string
		maxExec     uint
		minCoverage float64
	}{
		{"github.com/ajalab/congo/testdata", "Max2", 2, 1},
		{"github.com/ajalab/congo/testdata", "Min2", 2, 1},
		{"github.com/ajalab/congo/testdata", "Max3", 5, 1},
	}

	for _, tc := range testCases {
		conf := Config{
			PackageName: tc.packageName,
			FuncName:    tc.funcName,
		}

		prog, err := conf.Open()
		if err != nil {
			t.Fatalf("Config.Open: %v\n", err)
		}

		res, err := prog.Execute(tc.maxExec, tc.minCoverage)
		if err != nil {
			t.Fatalf("Program.Execute: %v\n", err)
		}
		if res.Coverage < tc.minCoverage {
			t.Fatalf("coverage could not be accomplished: expected %f, actual %f\n", tc.minCoverage, res.Coverage)
		}
	}
}
