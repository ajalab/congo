package testdata

import "fmt"

func branchPlus(x int32) {
	y := x * 2

	if 0 < y && y < 10 {
		fmt.Printf("y is in 0 ~ 10")
	} else {
		fmt.Printf("other")
	}
}
