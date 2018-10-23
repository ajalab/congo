package testdata

import "fmt"

func ArrayIncreasing(xs [5]int) {
	i := 0
	for ; i < len(xs)-1; i++ {
		if xs[i] >= xs[i+1] {
			break
		}
	}
	if i == len(xs)-1 {
		fmt.Println("xs is increasing")
	} else {
		fmt.Println("xs is not increasing")
	}
}
