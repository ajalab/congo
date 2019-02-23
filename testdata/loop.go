package testdata

import "fmt"

// LoopInt is a test case for checking loop.
// congo:maxexec 12
// congo:cover 1.0
func LoopInt(x int) {
	sum := 0

	for i := 0; i < x; i++ {
		sum = sum + i
	}

	if sum < 50 {
		fmt.Println("sum of 1 .. x is less than 50")
	} else {
		fmt.Println("sum of 1 .. x is greater than 50")
	}
}
