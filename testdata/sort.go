package testdata

// Sort2 returns values of a and b in ascending order.
// congo:maxexec 2
// congo:cover 1.0
func Sort2(a, b int) (x, y int) {
	if a < b {
		x, y = a, b
	} else {
		x, y = b, a
	}
	return
}

// USort2 returns values of unsigned integers a and b in ascending order.
// congo:maxexec 2
// congo:cover 1.0
func USort2(a, b uint) (x, y uint) {
	if a < b {
		x, y = a, b
	} else {
		x, y = b, a
	}
	return
}

// Sort3 returns values of a, b and c in ascending order.
// congo:maxexec 6
// congo:cover 1.0
func Sort3(a, b, c int) (x, y, z int) {
	if a < b {
		if c < a {
			x, y, z = c, a, b
		} else {
			if c < b {
				x, y, z = a, c, b
			} else {
				x, y, z = a, b, c
			}
		}
	} else {
		if c < b {
			x, y, z = c, b, a
		} else {
			if c < a {
				x, y, z = b, c, a
			} else {
				x, y, z = b, a, c
			}
		}
	}
	return
}

// USort3 returns values of unsigned integers a, b and c in ascending order.
// congo:maxexec 6
// congo:cover 1.0
func USort3(a, b, c uint) (x, y, z uint) {
	if a < b {
		if c < a {
			x, y, z = c, a, b
		} else {
			if c < b {
				x, y, z = a, c, b
			} else {
				x, y, z = a, b, c
			}
		}
	} else {
		if c < b {
			x, y, z = c, b, a
		} else {
			if c < a {
				x, y, z = b, c, a
			} else {
				x, y, z = b, a, c
			}
		}
	}
	return
}
