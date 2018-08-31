package testdata

import "fmt"

func BranchMultiple(x int32) {
	if x < 5 {
		fmt.Println("x is small")
	} else if 5 <= x && x < 10 {
		fmt.Println("x is medium")
	} else {
		fmt.Println("x is large")
	}
}
