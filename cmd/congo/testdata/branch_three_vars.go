package testdata

import "fmt"

func BranchThreeVars(x int32, y int32, z int32) {
	if x+y+z > 50 {
		fmt.Println("x + y + z is greater than 50")
	} else {
		fmt.Println("x + y + z is less than or equal to 50")
	}
}
