package testdata

import "fmt"

// BoolNot is a test case to check NOT operation.
func BoolNot(a bool) {
	if !a {
		fmt.Println("!a")
	} else {
		fmt.Println("a")
	}
}

// BoolAnd is a test case to check AND operation.
func BoolAnd(a, b bool) {
	if a && b {
		fmt.Println("a && b")
	} else {
		fmt.Println("not a && b")
	}
}

// BoolOr is a test case to check OR operation.
func BoolOr(a, b bool) {
	if a || b {
		fmt.Println("a || b")
	} else {
		fmt.Println("not a || b")
	}
}

// BoolAll3 is a test case to check multiple use of AND operation.
func BoolAll3(a, b, c bool) {
	if a && b && c {
		fmt.Println("a && b && c is true")
	} else {
		fmt.Println("a && b && c is false")
	}
}

// BoolAny3 is a test case to check multiple use of OR operation.
func BoolAny3(a, b, c bool) {
	if a || b || c {
		fmt.Println("a || b || c is true")
	} else {
		fmt.Println("a || b || c is false")
	}
}

// Bool3 is a test case to check boolean operations
func Bool3(a, b, c bool) {
	if a && !b && c {
		fmt.Println("a && !b && c")
	} else {
		fmt.Println("not a && !b && c")
	}
}
