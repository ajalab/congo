package testdata

import "fmt"

func BoolAllTrue(a, b, c bool) {
	if a && b && c {
		fmt.Println("a && b && c is true")
	} else {
		fmt.Println("a && b && c is false")
	}
}
