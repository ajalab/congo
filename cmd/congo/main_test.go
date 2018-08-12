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
		config := Config{
			PackageName: tc.packageName,
			FuncName:    tc.funcName,
		}

		program, err := config.Open()
		if err != nil {
			t.Fatalf("config.Open: %v", err)
		}

		if err = program.RunWithZeroValues(); err != nil {
			t.Errorf("program.RunWithZeroValue: %v", err)
		}
	}
}
