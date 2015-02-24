package p

func f() {
	if b {
		goto foo
	}
	goto bar

foo:
	f()
	f()
	f()
	goto bar

bar:
	baz()
	return
}

func baz()

var b bool
