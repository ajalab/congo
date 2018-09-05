package testdata

func Max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func Max3(a, b, c int) int {
	if a > b {
		if c > a {
			return c
		}
		return a

	} else {
		if c > b {
			return c
		}
		return b

	}
}

func Min2(a, b int) int {
	if a <= b {
		return a
	}
	return b
}
func Min3(a, b, c int) int {
	if a <= b {
		if c <= a {
			return c
		}
		return a

	} else {
		if c <= b {
			return c
		}
		return b

	}
}
