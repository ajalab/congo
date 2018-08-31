package testdata

import "fmt"

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
