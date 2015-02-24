package p

func f() {
	if b {
		goto ret
	}
	if b {
		goto ret
	}
	baz()

ret:
}

func f2() {
	if b {
		goto ret
	}
	if b {
		goto ret
	}
	baz()

ret:
	return
}

func baz()

var b bool
