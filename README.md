# Congo

![](assets/screenshot.png)

Congo is a test generation framework for [Go](https://golang.org/).
It adopts [concolic testing](https://en.wikipedia.org/wiki/Concolic_testing) to generate test cases that will
achieve better test coverage than randomly generated ones.

## Dependencies

Congo requires [Z3](https://github.com/Z3Prover/z3) to solve symbolic constraints.
The latest version is recommended.

## Install

```sh
$ go get -u github.com/ajalab/congo/...
```