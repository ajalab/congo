package testdata

import "fmt"

func plus(a, b int) int {
	return a + b
}

func UsePlus(a, b, c int) {
	x := plus(a, b)
	if x == 10 {
		y := plus(x, c)
		if y == 20 {
			fmt.Println("a + b == 10 and a + b + c == 20")
		}
	}
}
