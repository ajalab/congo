package testdata

import "fmt"

// BranchLessThan is a test case for checking < operator.
func BranchLessThan(x int32) {
	if x < 5 {
		fmt.Println("x is smaller than 5")
	} else {
		fmt.Println("other")
	}
}

// BranchAnd is a test case for checking && condition.
func BranchAnd(x int32) {
	if 0 < x && x < 5 {
		fmt.Println("x is in 0 ~ 5")
	} else {
		fmt.Println("other")
	}
}

// BranchMultiple is a test case for checking consecutive if statements.
func BranchMultiple(x int32) {
	if x < 5 {
		fmt.Println("x is small")
	} else if 5 <= x && x < 10 {
		fmt.Println("x is medium")
	} else {
		fmt.Println("x is large")
	}
}

// BranchPhi is a test case for checking φ-function.
func BranchPhi(x int) {
	y := 0

	// 0
	if x > 5 {
		y = 1
	}

	// 1
	if y == 1 {
		fmt.Println("x is greater than 5")
	} else {
		fmt.Println("x is less than or equal to 5")
	}
}

// BranchPhi2 is a test case for checking φ-function.
func BranchPhi2(x int) {
	var y int

	if x < 5 {
		y = x * 10
	} else {
		y = x + 2
	}

	if y == 30 {
		fmt.Println("y is 30")
	} else {
		fmt.Println("y is not 30")
	}
}

// BranchThreeVars is a test case for checking multiple arguments.
func BranchThreeVars(x int32, y int32, z int32) {
	if x+y+z > 50 {
		fmt.Println("x + y + z is greater than 50")
	} else {
		fmt.Println("x + y + z is less than or equal to 50")
	}
}

// BranchTenVars is a test case for checking multiple arguments.
func BranchTenVars(a int32, b int32, c int32, d int32, e int32, f int32, g int32, h int32, i int32, j int32) {
	if a+b+c+d+e+f+g+h+i+j > 50 {
		fmt.Println("a+b+c+d+e+f+g+h+i+j is greater than 50")
	} else {
		fmt.Println("a+b+c+d+e+f+g+h+i+j is less than or equal to 50")
	}
}

// BranchSwitch is a test case for checking a switch statement.
func BranchSwitch(x int) {
	switch x {
	case 0:
		fmt.Println("x is 0")
	case 1:
		fmt.Println("x is 1")
	case 2:
		fmt.Println("x is 2")
	default:
		fmt.Println("x is neither 0, 1, nor 2")
	}
}
