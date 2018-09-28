package testdata

import "fmt"

// QuadraticEq1 reports whether x is a solution of x^2 - 3x + 4.
func QuadraticEq1(x int) {
	if x*x-3*x-4 == 0 {
		if x > 0 {
			fmt.Printf("%d is a solution of x^2 - 3x + 4 (the greater one)\n", x)
		} else {
			fmt.Printf("%d is a solution of x^2 - 3x + 4 (the smaller one)\n", x)
		}
	} else {
		fmt.Printf("%d is not a solution of x^2 - 3x + 4\n", x)
	}
}

func AddOverflow(n uint8) {
	if n+50 == 32 {
		fmt.Println("n + 50 == 32")
	} else {
		fmt.Println("n + 50 =/= 32")
	}
}
