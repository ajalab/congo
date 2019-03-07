package testdata

import "fmt"

// PointerIsNil is a test case to check nil handling.
// congo:maxexec 2
// congo:cover 1.0
func PointerIsNil(a *int) {
	if a == nil {
		fmt.Println("a is nil")
	} else {
		fmt.Println("a is not nil")
	}
}

// PointerIsNotNil1 is a test case to check nil handling.
// congo:maxexec 2
// congo:cover 1.0
func PointerIsNotNil1(a *int) bool {
	if a == nil {
		return false
	}
	return true
}

// PointerIsNotNil2 is a test case to check nil handling.
// congo:maxexec 2
// congo:cover 1.0
func PointerIsNotNil2(a *int) bool {
	if a != nil {
		return true
	}
	return false
}

// PointerDeref is a test case to check pointer indirection.
// congo:maxexec 3
// congo:cover 1.0
func PointerDeref(a *int) {
	if *a < 5 {
		fmt.Println("*a is less than 5")
	} else {
		fmt.Println("*a is greater than or equal to 5")
	}
}

// PointerDeref2 is a test case to check pointer indirection.
// congo:maxexec 3
// congo:cover 1.0
func PointerDeref2(a *int) {
	if 0 < *a && *a < 5 {
		fmt.Println("0 < *a < 5")
	} else {
		fmt.Println("not 0 < *a < 5")
	}
}

// PointerDeref3 is a test case to check pointer indirection.
// congo:maxexec 6
// congo:cover 1.0
func PointerDeref3(a, b *int) {
	if *a > 0 && *b > 0 && *a+*b == 5 {
		fmt.Println("*a > 0 and *b > 0 and *a + *b = 5")
	} else {
		fmt.Println("no")
	}
}

// PointerStore is a test case to check storing a value.
// congo:maxexec 5
// congo:cover 1.0
func PointerStore(a *int) {
	if *a > 0 {
		*a = *a * 2
	}
	if *a > 2 {
		*a = *a + 1
	}
	if *a == 11 {
		fmt.Println("found")
	}
}

// PointerStore2 is a test case to check storing a value.
// congo:maxexec 3
// congo:cover 1.0
func PointerStore2(a *int) bool {
	*a = *a * 2
	if *a == 100 {
		return false
	}
	return true
}

/*
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
*/
