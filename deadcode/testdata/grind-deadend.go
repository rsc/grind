package p

var cond bool

func f() {
	f()
	return
	goto bad
bad:
}
