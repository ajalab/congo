package testdata

import "fmt"

func branch_and(x int32) {
	if 0 < x && x < 5 {
		fmt.Printf("x is in 0 ~ 5")
	} else {
		fmt.Printf("other")
	}
}
