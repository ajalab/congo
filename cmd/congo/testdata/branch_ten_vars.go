package testdata

import "fmt"

func BranchTenVars(a int32, b int32, c int32, d int32, e int32, f int32, g int32, h int32, i int32, j int32) {
	if a+b+c+d+e+f+g+h+i+j > 50 {
		fmt.Println("a+b+c+d+e+f+g+h+i+j is greater than 50")
	} else {
		fmt.Println("a+b+c+d+e+f+g+h+i+j is less than or equal to 50")
	}
}
