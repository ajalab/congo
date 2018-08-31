package testdata

import "fmt"

func BranchSwitch(x int) {
	switch x {
	case 0:
		fmt.Println("x is 0")
	case 1:
		fmt.Println("x is 1")
	case 2:
		fmt.Println("x is 2")
	default:
		fmt.Println("x is neither 0, 1, nor 2")
	}
}
