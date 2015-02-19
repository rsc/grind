package p

func f() {
L:
	select {
	case lhs1 := <-rhs1:
		body1()
		if brk {
			break
		}
		more1()
	case <-rhs2:
		body2()
		if brk {
			break L
		}
		more2()
	case lhs3 = <-rhs3:
		body3()
	case lhs4 <- rhs4:
		body4()
	default:
		bodyDefault()
	}
}
