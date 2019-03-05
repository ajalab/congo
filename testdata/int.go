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

// AndOr1 is a test case to check AND and OR.
// congo:maxexec 5
// congo:cover 1.0
func AndOr1(a, b, c int64) bool {
	if a > 0 && b > 0 && c > 0 {
		if a|b|c == 0xfff && a&b == 0 && b&c == 0 && c&a == 0 {
			return true
		}
	}
	return false
}

// Xor1 is a test case to check XOR.
// congo:maxexec 5
// congo:cover 1.0
func Xor1(a, b, c int64) bool {
	if a > 0 && b > 0 && c > 0 && a^b^c == 0xffff {
		return true
	}
	return false
}

// AndNot1 is a test case to check AND_NOT.
// congo:maxexec 2
// congo:cover 1.0
func AndNot1(a int64) bool {
	if 0xfa03&^a == 0xf000 {
		return true
	}
	return false
}

// ShiftLeft1 is a test case to check SHL.
// congo:maxexec 2
// congo:cover 1.0
func ShiftLeft1(a int64) bool {
	if a<<2 == 20 {
		return true
	}
	return false
}

// ShiftLeft2 is a test case to check SHL.
// !congo:maxexec 3
// !congo:cover 1.0
func ShiftLeft2(a int8) bool {
	if a<<5 == -96 && a < 0 {
		return true
	}
	return false
}
