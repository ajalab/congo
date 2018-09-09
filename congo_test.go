package congo

import (
	"testing"
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
		values := make([]interface{}, n)
		for i, symbol := range prog.symbols {
			values[i] = zero(symbol.Type())
		}

		if _, err = prog.Run(values); err != nil {
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
		{"github.com/ajalab/congo/testdata", "Max3", 4, 1},
		{"github.com/ajalab/congo/testdata", "Min3", 4, 1},
		{"github.com/ajalab/congo/testdata", "UMax2", 2, 1},
		{"github.com/ajalab/congo/testdata", "UMin2", 2, 1},
		{"github.com/ajalab/congo/testdata", "UMax3", 4, 1},
		{"github.com/ajalab/congo/testdata", "UMin3", 4, 1},
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
			t.Fatalf("%+v\ncoverage could not be accomplished: expected %f, actual %f\n", tc, tc.minCoverage, res.Coverage)
		}
	}
}
