package testdata

import "fmt"

// PLessThan5 is a test case to check pointer indirection.
func PLessThan5(a *int) {
	if *a < 5 {
		fmt.Println("*a is less than 5")
	} else {
		fmt.Println("*a is greater than or equal to 5")
	}
}
