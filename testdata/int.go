package testdata

import "fmt"

// IntNeg is a test case to check integer negation.
// congo:maxexec 2
// congo:cover 1.0
func IntNeg(a int64) {
	if -a == 5 {
		fmt.Println("-a == 5")
	} else {
		fmt.Println("-a != 5")
	}
}

// IntNegUnsigned is a test case to check unsigned integer negation.
// congo:maxexec 2
// congo:cover 1.0
func IntNegUnsigned(a uint64) {
	if -a == 5 {
		fmt.Println("-a == 5")
	} else {
		fmt.Println("-a != 5")
	}
}

// AddOverflow is a test case to check addition that may cause overflow.
// congo:maxexec 2
// congo:cover 1.0
func AddOverflow(n uint8) {
	if n+50 == 32 {
		fmt.Println("n + 50 == 32")
	} else {
		fmt.Println("n + 50 != 32")
	}
}

// SubOverflow is a test case to check subtraction that may cause overflow.
// congo:maxexec 2
// congo:cover 1.0
func SubOverflow(n uint8) {
	if n-50 == 244 {
		fmt.Println("n - 50 == 244")
	} else {
		fmt.Println("n - 50 != 244")
	}
}

// Mod1 is a test case to check mod.
// congo:maxexec 3
// congo:cover 1.0
func Mod1(n int) bool {
	if n > 2 && n%4 == 2 {
		return true
	}
	return false
}

// Mod2 is a test case to check mod.
// congo:maxexec 3
// congo:cover 1.0
func Mod2(n int) bool {
	if n < 0 && n%4 == -2 {
		return true
	}
	return false
}
