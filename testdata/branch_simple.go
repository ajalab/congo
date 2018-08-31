package testdata

import "fmt"

func BranchSimple(x int32) {
	if x < 5 {
		fmt.Println("x is smaller than 5")
	} else {
		fmt.Println("other")
	}
}
