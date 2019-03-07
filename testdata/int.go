package testdata

import "fmt"

// IntNotEqual is a test case tot check != operator.
// congo:maxexec 2
// congo:cover 1.0
func IntNotEqual(a int) bool {
	if a != 100 {
		return true
	}
	return false
}

// Neg1 is a test case to check integer negation.
// congo:maxexec 2
// congo:cover 1.0
func Neg1(a int64) {
	if -a == 5 {
		fmt.Println("-a == 5")
	} else {
		fmt.Println("-a != 5")
	}
}

// Neg2 is a test case to check unsigned integer negation.
// congo:maxexec 2
// congo:cover 1.0
func Neg2(a uint64) {
	if -a == 5 {
		fmt.Println("-a == 5")
	} else {
		fmt.Println("-a != 5")
	}
}

// Compl1 is a test case to check bitwise complement of signed integer.
// congo:maxexec 2
// congo:cover 1.0
func Compl1(a int) bool {
	if ^a == 10 {
		return true
	}
	return false
}

// Compl2 is a test case to check bitwise complement of unsigned integer.
// congo:maxexec 2
// congo:cover 1.0
func Compl2(a uint) bool {
	if ^a == 10 {
		return true
	}
	return false
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
func ShiftLeft1(n uint8) bool {
	if uint8(1)<<n == 128 {
		return true
	}
	return false
}

// ShiftLeft2 is a test case to check SHL.
// congo:maxexec 2
// congo:cover 1.0
func ShiftLeft2(n uint8) bool {
	if int8(1)<<n == -128 {
		return true
	}
	return false
}

// ShiftLeft3 is a test case to check SHL.
// congo:maxexec 2
// congo:cover 1.0
func ShiftLeft3(n uint8) bool {
	if int8(-1)<<n == -64 {
		return true
	}
	return false
}

// ShiftLeft4 is a test case to check SHL.
// congo:maxexec 2
// congo:cover 1.0
func ShiftLeft4(n uint8) bool {
	if uint16(1)<<n == 4096 {
		return true
	}
	return false
}

// ShiftLeft5 is a test case to check SHL.
// congo:maxexec 2
// congo:cover 1.0
func ShiftLeft5(n uint16) bool {
	if int8(1)<<n == -128 {
		return true
	}
	return false
}

// ShiftRight1 is a test case to check SHR.
// congo:maxexec 2
// congo:cover 1.0
func ShiftRight1(n uint8) bool {
	if uint8(0x80)>>n == 0x4 {
		return true
	}
	return false
}

// ShiftRight2 is a test case to check SHR.
// congo:maxexec 2
// congo:cover 1.0
func ShiftRight2(n uint8) bool {
	if int8(-0x80)>>n == -4 {
		return true
	}
	return false
}
