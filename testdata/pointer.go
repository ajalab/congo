package testdata

import "fmt"

// PointerIsNil is a test case to check nil handling
func PointerIsNil(a *int) {
	if a == nil {
		fmt.Println("a is nil")
	} else {
		fmt.Println("a is not nil")
	}
}

// PointerDeref is a test case to check pointer indirection.
func PointerDeref(a *int) {
	if *a < 5 {
		fmt.Println("*a is less than 5")
	} else {
		fmt.Println("*a is greater than or equal to 5")
	}
}

// PointerDoubleDeref is a test case to check nested pointer indirection.
func PointerDoubleDeref(a **int) {
	if **a < 5 {
		fmt.Println("**a is less than 5")
	} else {
		fmt.Println("**a is greater than or equal to 5")
	}
}

var PVar int = 3

// PointerEquals is a test case to check pointer equivalence.
func PointerEquals(a *int) {
	if a == &PVar {
		fmt.Println("&a == &PVar")
	} else {
		fmt.Println("&a != &PVar")
	}
}
