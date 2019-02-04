# Congo internals

## Test Runner

Given a function signature,
Congo first generates a test runner which calls the function.
For example, if the target function in the package `myapp` has a signature like `Foo(a, b int) int`,
the following runner is generated.

```go
package main

import "myapp"
import "github.com/ajalab/congo/symbol"

func main() {
	actual0 := myapp.Foo(symbol.Symbols[0].(int), symbol.Symbols[1].(int))
	symbol.TestAssert(actual0 == symbol.RetVals[0].(int))
}
```

The second package `github.com/ajalab/congo/symbol` contains variables and functions that Congo specially treats.
For each execution, Congo substitutes values got by solving constraints for `symbol.Symbols`, which is a slice of `interface{}`.

A test runner is also used as a template for test codes to generate.
`symbol.TestAssert` does nothing at execution time but is replaced with assertions in a generated code. The AST node `symbol.RetVals[i].(type)` is replaced with a variable that contains a value
returned by the target function during concolic execution.

## Run

## Strategy