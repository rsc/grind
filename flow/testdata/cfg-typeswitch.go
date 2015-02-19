package p

func f() {
L:
	switch x := expr.(type) {
	case case1, case2, case3:
		body123()
	case case4, case5, case6:
		body456()
	case case7:
		body7()
		if brk {
			break
		}
		more7()
	default:
		bodyDefault()
	case case8:
		body8()
		if brkL {
			break L
		}
		more8()
	}
}
