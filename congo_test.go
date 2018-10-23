package congo

import (
	"testing"
)

func TestRun(t *testing.T) {
	testCases := []struct {
		packageName string
		funcName    string
	}{
		{"github.com/ajalab/congo/testdata", "BranchLessThan"},
		{"github.com/ajalab/congo/testdata", "BranchAnd"},
		{"github.com/ajalab/congo/testdata", "BranchThreeVars"},
		{"github.com/ajalab/congo/testdata", "BranchTenVars"},
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

type executeTestCase struct {
	packageName string
	funcName    string
	maxExec     uint
	minCoverage float64
}

func testExecute(testCases []executeTestCase, t *testing.T) {
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

func TestExecuteBranch(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "BranchLessThan", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchAnd", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchMultiple", 4, 1},
		{"github.com/ajalab/congo/testdata", "BranchThreeVars", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchTenVars", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchSwitch", 4, 1},
	}
	testExecute(testCases, t)
}

func TestExecuteMaxMin(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "Max2", 2, 1},
		{"github.com/ajalab/congo/testdata", "Min2", 2, 1},
		{"github.com/ajalab/congo/testdata", "Max3", 4, 1},
		{"github.com/ajalab/congo/testdata", "Min3", 4, 1},
		{"github.com/ajalab/congo/testdata", "UMax2", 2, 1},
		{"github.com/ajalab/congo/testdata", "UMin2", 2, 1},
		{"github.com/ajalab/congo/testdata", "UMax3", 4, 1},
		{"github.com/ajalab/congo/testdata", "UMin3", 4, 1},
	}
	testExecute(testCases, t)
}

func TestExecuteString(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "IsABC", 2, 1},
		{"github.com/ajalab/congo/testdata", "IsABCIfConcatenated", 2, 1},
	}
	testExecute(testCases, t)
}

func TestExecuteCall(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "UsePlus", 3, 1},
	}
	testExecute(testCases, t)
}
