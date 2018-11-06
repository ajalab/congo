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

// PointerDeref2 is a test case to check pointer indirection.
func PointerDeref2(a *int) {
	if 0 < *a && *a < 5 {
		fmt.Println("0 < *a < 5")
	} else {
		fmt.Println("not 0 < *a < 5")
	}
}

// PointerDeref3 is a test case to check pointer indirection.
func PointerDeref3(a, b *int) {
	if *a > 0 && *b > 0 && *a+*b == 5 {
		fmt.Println("*a > 0 and *b > 0 and *a + *b = 5")
	} else {
		fmt.Println("no")
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
