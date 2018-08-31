package testdata

import "fmt"

func BranchAnd(x int32) {
	if 0 < x && x < 5 {
		fmt.Println("x is in 0 ~ 5")
	} else {
		fmt.Println("other")
	}
}
