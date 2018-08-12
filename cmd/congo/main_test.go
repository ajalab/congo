package main

import "testing"

func TestRunWithZeroValue(t *testing.T) {
	testCases := []struct {
		packageName string
		funcName    string
	}{
		{"github.com/ajalab/congo/cmd/congo/testdata", "BranchSimple"},
		{"github.com/ajalab/congo/cmd/congo/testdata", "BranchAnd"},
		{"github.com/ajalab/congo/cmd/congo/testdata", "BranchThreeVars"},
		{"github.com/ajalab/congo/cmd/congo/testdata", "BranchTenVars"},
		{"github.com/ajalab/congo/cmd/congo/testdata", "BranchStruct"},
	}

	for _, tc := range testCases {
		conf := config{
			packageName: tc.packageName,
			funcName:    tc.funcName,
		}

		prog, err := conf.Open()
		if err != nil {
			t.Fatalf("config.Open: %v", err)
		}

		if err = prog.RunWithZeroValues(); err != nil {
			t.Errorf("prog.RunWithZeroValue: %v", err)
		}
	}
}
