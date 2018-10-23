package testdata

import "fmt"

func SliceIsLen3(xs []int) {
	if len(xs) == 3 {
		fmt.Println("len(xs) == 3")
	} else {
		fmt.Println("len(xs) != 3")
	}
}

func SliceIncreasing(xs []int) {
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
