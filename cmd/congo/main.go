package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ajalab/congo"
)

func main() {
	var packageName, funcName string
	if len(os.Args) <= 2 {
		packageName = "github.com/ajalab/congo/cmd/congo/testdata"
		funcName = "BranchThreeVars"
	} else {
		packageName = os.Args[1]
		funcName = os.Args[2]

	}
	conf := congo.Config{
		PackageName: packageName,
		FuncName:    funcName,
		MaxExec:     3,
	}

	prog, err := conf.Open()
	if err != nil {
		log.Fatalf("Config.Open: %v", err)
	}

	if err = prog.Execute(); err != nil {
		fmt.Println("failed: ", err)
	}
}
