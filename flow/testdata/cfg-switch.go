package p

func f() {
L:
	switch expr {
	case case1, case2, case3:
		body123()
	case case4, case5, case6:
		body456()
		fall()
		fallthrough
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

	switch expr {
	default:
		bodyDefault()
		fallthrough
	case case1, case2, case3:
		body123()
	}
}
