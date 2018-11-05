package main

import (
	"flag"
	"fmt"
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
	verbose     = flag.Bool("v", false, "verbose output (debug info)")
	funcName    = flag.String("f", "", "name of the target function")
	runner      = flag.String("r", "", "test template")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "package must be specified after flags")
		flag.Usage()
		return
	}
	if *funcName == "" {
		fmt.Fprintln(os.Stderr, "function name must be specified by -f option")
		flag.Usage()
		return
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	packageName := flag.Arg(0)
	targetPackage, err := congo.LoadTargetPackage(packageName)
	if err != nil {
		log.Fatalf("failed to load package %s: %+v", packageName, err)
	}

	runnerPackagePath := *runner
	if runnerPackagePath == "" {
		runnerPackagePath, err = congo.GenerateRunner(targetPackage, *funcName)
		if err != nil {
			log.Fatalf("failed to generate a runner: %v", err)
		}
		defer os.Remove(runnerPackagePath)
	}

	prog, err := congo.Load(targetPackage.PkgPath, runnerPackagePath, *funcName)
	if err != nil {
		log.Fatalf("failed to load: %+v", err)
	}
	if *verbose {
		prog.DumpRunnerAST(os.Stderr)
		prog.DumpRunnerSSA(os.Stderr)
	}

	result, err := prog.Execute(*maxExec, *minCoverage)
	if err != nil {
		log.Fatalf("failed to perform concolic execution: %+v", err)
	}
	f, err := result.GenerateTest()
	if err != nil {
		log.Fatalf("failed to generate test: %+v", err)
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
