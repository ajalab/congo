package testdata

import "fmt"

func BranchSimple(x int32) {
	if x < 5 {
		fmt.Printf("x is smaller than 5")
	} else {
		fmt.Printf("other")
	}
}
