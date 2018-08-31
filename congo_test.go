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
