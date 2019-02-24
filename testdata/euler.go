package testdata

// Problem9 solves Problem 9 in Project Euler.
// congo:cover 1.0
// congo:maxexec 9
func Problem9(a, b, c uint) uint {
	if 0 < a && a < 1000 && 0 < b && b < 1000 && 0 < c && c < 1000 {
		if a+b+c == 1000 && a*a+b*b == c*c {
			return a * b * c
		}
	}
	return 0
}
