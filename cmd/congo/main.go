package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/ajalab/congo"
)

var (
	cpuProfile  = flag.String("cpuprofile", "", "write cpu profile to file")
	minCoverage = flag.Float64("coverage", 1.0, "minimum coverage")
	maxExec     = flag.Uint("maxexec", 10, "maximum execution time")
)

func main() {
	flag.Parse()

	var packageName, funcName string
	if flag.NArg() < 2 {
		packageName = "github.com/ajalab/congo/testdata"
		funcName = "BranchSimple"
	} else {
		packageName = flag.Arg(0)
		funcName = flag.Arg(1)
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	conf := congo.Config{
		PackageName: packageName,
		FuncName:    funcName,
	}

	prog, err := conf.Open()
	if err != nil {
		log.Fatalf("Config.Open: %v", err)
	}

	if result, err := prog.Execute(*maxExec, *minCoverage); err != nil {
		fmt.Println("failed: ", err)
	} else {
		fmt.Println(result)
	}

}
