package testdata

import "fmt"

func BranchPhi2(x int) {
	y := 0

	// 0
	if x > 5 {
		y = 1
	}

	// 1
	if y == 1 {
		fmt.Println("x is greater than 5")
	} else {
		fmt.Println("x is less than or equal to 5")
	}
}
