package testdata

func Max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Max3(a, b, c int) int {
	// 0
	if a > b {
		// 1
		if c > a {
			// 3
			return c
		}
		// 4
		return a

	} else {
		// 2
		if c > b {
			// 5
			return c
		}
		// 6
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

func UMax2(a, b uint) uint {
	if a > b {
		return a
	}
	return b
}

func UMax3(a, b, c uint) uint {
	// 0
	if a > b {
		// 1
		if c > a {
			// 3
			return c
		}
		// 4
		return a

	} else {
		// 2
		if c > b {
			// 5
			return c
		}
		// 6
		return b
	}
}

func UMin2(a, b uint) uint {
	if a <= b {
		return a
	}
	return b
}

func UMin3(a, b, c uint) uint {
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
