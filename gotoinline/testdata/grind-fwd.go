package p

var cond bool

func f() {
	if cond {
		goto bad
	}
	return

bad:
	println(x)
}

var x int
