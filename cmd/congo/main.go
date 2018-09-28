package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"go/format"
	"go/token"

	"github.com/ajalab/congo"
)

var (
	cpuProfile  = flag.String("cpuprofile", "", "write cpu profile to file")
	minCoverage = flag.Float64("coverage", 1.0, "minimum coverage")
	maxExec     = flag.Uint("maxexec", 10, "maximum execution time")
	o           = flag.String("o", "", "destination path for generated test code")
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

	result, err := prog.Execute(*maxExec, *minCoverage)
	if err != nil {
		log.Fatalf("failed to perform concolic execution: %v", err)
	}
	f, err := result.GenerateTest()
	if err != nil {
		log.Fatalf("failed to generate test: %v", err)
	}

	dest := os.Stdout
	if *o != "" {
		log.Println("save to", *o)
		dest, err = os.Create(*o)
		if err != nil {
			log.Fatalf("faled to open the destination file: %v", err)
		}
	}
	format.Node(dest, token.NewFileSet(), f)
}
