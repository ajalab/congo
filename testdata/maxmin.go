package testdata

// Max2 returns the largest of a and b.
// congo:maxexec 2
// congo:cover 1.0
func Max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Max3 returns the largest of a, b and c.
// congo:maxexec 4
// congo:cover 1.0
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

	}
	// 2
	if c > b {
		// 5
		return c
	}
	// 6
	return b
}

// Min2 returns the smallest of a and b.
// congo:maxexec 2
// congo:cover 1.0
func Min2(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// Min3 returns the smallest of a, b and c.
// congo:maxexec 4
// congo:cover 1.0
func Min3(a, b, c int) int {
	if a <= b {
		if c <= a {
			return c
		}
		return a

	}
	if c <= b {
		return c
	}
	return b

}

// UMax2 returns the largest of unsigned integers a and b.
// congo:maxexec 2
// congo:cover 1.0
func UMax2(a, b uint) uint {
	if a > b {
		return a
	}
	return b
}

// UMax3 returns the largest of unsigned integers a, b and c.
// congo:maxexec 4
// congo:cover 1.0
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

	}
	// 2
	if c > b {
		// 5
		return c
	}
	// 6
	return b

}

// UMin2 returns the smallest of unsigned integers a and b.
// congo:maxexec 2
// congo:cover 1.0
func UMin2(a, b uint) uint {
	if a <= b {
		return a
	}
	return b
}

// UMin3 returns the smallest of unsigned integers a, b and c.
// congo:maxexec 4
// congo:cover 1.0
func UMin3(a, b, c uint) uint {
	if a <= b {
		if c <= a {
			return c
		}
		return a

	}
	if c <= b {
		return c
	}
	return b
}
