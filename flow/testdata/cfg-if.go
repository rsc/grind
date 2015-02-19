package p

func f() {
	f()
	if x {
		g()
	}
	h()
	if x {
		i()
	} else {
		j()
	}
	k()
}
