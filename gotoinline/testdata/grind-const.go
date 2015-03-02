package p

var cond bool

func f() {
	if cond {
		goto unary
	}
	goto ret

unary:
	f()

ret:
	f()
	return
}
