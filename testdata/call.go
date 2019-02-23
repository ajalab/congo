package testdata

import "fmt"

func plus(a, b int) int {
	return a + b
}

// UsePlus is a test case for checking function calls.
// congo:maxexec 3
// congo:cover 1.0
func UsePlus(a, b, c int) {
	x := plus(a, b)
	if x == 10 {
		y := plus(x, c)
		if y == 20 {
			fmt.Println("a + b == 10 and a + b + c == 20")
		}
	}
}

/*
func factor(n int) int {
	if n > 1 {
		return n * factor(n-1)
	}
	return 1

}

func Factor5040(n int) {
	f := factor(n)
	if f == 5040 {
		fmt.Println("n! == 5040")
	} else {
		fmt.Println("n! != 5040")
	}
}
*/
