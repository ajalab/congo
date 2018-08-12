package testdata

import "fmt"

func branch_multiple(x int32) {
	if x < 5 {
		fmt.Printf("x is small")
	} else if 5 <= x && x < 10 {
		fmt.Printf("x is medium")
	} else {
		fmt.Printf("x is large")
	}
}
