package p

func f() {
	if cond {
		goto bad
	}
	return

	// comment

	// more comment
bad:
	f()
}

var cond bool
