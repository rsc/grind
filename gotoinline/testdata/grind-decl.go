package p

var cond bool

func f() {
	if cond {
		goto bad
	}
	return

bad:
	if cond {
		x := 1
		println(x)
	}
}
