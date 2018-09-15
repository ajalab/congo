package testdata

func Sort2(a, b int) (x, y int) {
	if a < b {
		x, y = a, b
	} else {
		x, y = b, a
	}
	return
}

func USort2(a, b uint) (x, y uint) {
	if a < b {
		x, y = a, b
	} else {
		x, y = b, a
	}
	return
}

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
