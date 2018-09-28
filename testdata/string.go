package testdata

import "fmt"

// IsABC is a function that checks if s is equal to "ABC".
func IsABC(s string) {
	if s == "ABC" {
		fmt.Println("s is ABC")
	} else {
		fmt.Println("s is not ABC")
	}
}

// IsABCIfConcatenated is a function that checks if the concatenated string s1 + s2 is equal to "ABC".
func IsABCIfConcatenated(s1, s2 string) {
	if s1+s2 == "ABC" {
		fmt.Println("s1 + s2 is ABC")
	} else {
		fmt.Println("s1 + s2 is not ABC")
	}
}
