package testdata

import "fmt"

func BranchPhi(x int) {
	var y int

	if x < 5 {
		y = x * 10
	} else {
		y = x + 2
	}

	if y == 30 {
		fmt.Println("y is 30")
	} else {
		fmt.Println("y is not 30")
	}
}
