package congo

import (
	"fmt"
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
		prog, err := Load(tc.packageName, tc.funcName)
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
		t.Run(fmt.Sprintf("%s.%s", tc.packageName, tc.funcName), func(t *testing.T) {
			prog, err := Load(tc.packageName, tc.funcName)
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
		})
	}
}

func TestExecuteBool(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "BoolNot", 2, 1},
		{"github.com/ajalab/congo/testdata", "BoolAnd", 3, 1},
		{"github.com/ajalab/congo/testdata", "BoolOr", 2, 1},
		{"github.com/ajalab/congo/testdata", "BoolAll3", 4, 1},
		{"github.com/ajalab/congo/testdata", "BoolAny3", 2, 1},
		{"github.com/ajalab/congo/testdata", "Bool3", 3, 1},
	}
	testExecute(testCases, t)
}

func TestExecuteInt(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "IntNeg", 2, 1},
		{"github.com/ajalab/congo/testdata", "IntNegUnsigned", 2, 1},
		{"github.com/ajalab/congo/testdata", "AddOverflow", 2, 1},
		{"github.com/ajalab/congo/testdata", "SubOverflow", 2, 1},
	}
	testExecute(testCases, t)
}

func TestExecuteBranch(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "BranchLessThan", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchAnd", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchMultiple", 4, 1},
		{"github.com/ajalab/congo/testdata", "BranchPhi", 2, 1},
		{"github.com/ajalab/congo/testdata", "BranchPhi2", 3, 1},
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

func TestExecutePointer(t *testing.T) {
	testCases := []executeTestCase{
		{"github.com/ajalab/congo/testdata", "PointerDeref", 3, 1},
		{"github.com/ajalab/congo/testdata", "PointerDeref2", 3, 1},
	}
	testExecute(testCases, t)
}
