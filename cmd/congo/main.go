package main

import (
	"log"
	"os"
)

func main() {
	var packageName, funcName string
	if len(os.Args) <= 2 {
		packageName = "github.com/ajalab/congo/testdata"
		funcName = "BranchThreeVars"
	} else {
		packageName = os.Args[1]
		funcName = os.Args[2]

	}
	config := Config{
		PackageName: packageName, // "github.com/ajalab/congo/testdata",
		FuncName:    funcName,    // "BranchTenVars",
	}

	program, err := config.Open()
	if err != nil {
		log.Fatalf("config.Open: %v", err)
	}

	program.RunWithZeroValues()
}
