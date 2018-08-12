package testdata

import (
	"fmt"
)

func loopSlice(xs []int) {
	var n = 0

	for _, x := range xs {
		n = n + x
	}

	if n > 10 {
		fmt.Println("sum(xs) is greater than 10")
	} else {
		fmt.Println("sum(xs) is less than or equal to 10")
	}
}
