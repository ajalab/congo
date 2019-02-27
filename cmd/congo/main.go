package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"go/format"
	"go/token"

	"github.com/ajalab/congo"
	"github.com/ajalab/congo/log"
)

var (
	cpuProfile  = flag.String("cpuprofile", "", "write cpu profile to file")
	minCoverage = flag.Float64("coverage", 0.0, "minimum coverage")
	maxExec     = flag.Uint("maxexec", 0, "maximum execution time")
	o           = flag.String("o", "", "destination path for generated test code")
	ssa         = flag.Bool("ssa", false, "dump SSA")
	ast         = flag.Bool("ast", false, "dump AST")
	logLevel    = flag.String("log", "info", "log level (debug, info, error, disabled)")
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
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Error.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	log.SetLevelByName(*logLevel)

	targetPackagePath := flag.Arg(0)
	var funcNames []string
	if *funcName != "" {
		funcNames = []string{*funcName}
	}
	config := &congo.Config{
		FuncNames: funcNames,
		ExecuteOption: congo.ExecuteOption{
			MaxExec:     *maxExec,
			MinCoverage: *minCoverage,
		},
	}
	c, err := congo.Load(config, targetPackagePath)
	if err != nil {
		log.Error.Fatalf("failed to load: %+v", err)
	}
	if *ssa {
		c.DumpSSA(os.Stderr)
		return
	}
	if *ast {
		c.DumpRunnerAST(os.Stderr)
		return
	}

	dest := os.Stdout
	if *o != "" {
		log.Info.Print("save to", *o)
		dest, err = os.Create(*o)
		if err != nil {
			log.Error.Fatalf("faled to open the destination file: %v", err)
		}
	}
	for _, name := range c.Funcs() {
		result, err := c.Execute(name)
		if err != nil {
			log.Error.Fatalf("failed to perform concolic execution: %+v", err)
		}
		f, err := result.GenerateTest()
		if err != nil {
			log.Error.Fatalf("failed to generate test: %+v", err)
		}
		format.Node(dest, token.NewFileSet(), f)
	}
}
