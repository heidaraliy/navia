package ui

func Split(width int) (left, right int) {
	if width < 80 {
		left = width / 2
	} else {
		left = width * 45 / 100
	}
	if left < 28 {
		left = 28
	}
	if left > width-24 {
		left = width - 24
	}
	right = width - left
	return left, right
}
