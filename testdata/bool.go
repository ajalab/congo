package testdata

import "fmt"

// BoolNot is a test case to check NOT operation.
// congo:maxexec 2
// congo:cover 1.0
func BoolNot(a bool) {
	if !a {
		fmt.Println("!a")
	} else {
		fmt.Println("a")
	}
}

// BoolAnd is a test case to check AND operation.
// congo:maxexec 3
// congo:cover 1.0
func BoolAnd(a, b bool) {
	if a && b {
		fmt.Println("a && b")
	} else {
		fmt.Println("not a && b")
	}
}

// BoolOr is a test case to check OR operation.
// congo:maxexec 2
// congo:cover 1.0
func BoolOr(a, b bool) {
	if a || b {
		fmt.Println("a || b")
	} else {
		fmt.Println("not a || b")
	}
}

// BoolAll3 is a test case to check multiple use of AND operation.
// congo:maxexec 4
// congo:cover 1.0
func BoolAll3(a, b, c bool) {
	if a && b && c {
		fmt.Println("a && b && c is true")
	} else {
		fmt.Println("a && b && c is false")
	}
}

// BoolAny3 is a test case to check multiple use of OR operation.
// congo:maxexec 2
// congo:cover 1.0
func BoolAny3(a, b, c bool) {
	if a || b || c {
		fmt.Println("a || b || c is true")
	} else {
		fmt.Println("a || b || c is false")
	}
}

// Bool3 is a test case to check boolean operations
// congo:maxexec 3
// congo:cover 1.0
func Bool3(a, b, c bool) {
	if a && !b && c {
		fmt.Println("a && !b && c")
	} else {
		fmt.Println("not a && !b && c")
	}
}
