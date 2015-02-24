package p

func f() {
bad:
}

func f1() {
	if b {
	bad:
		println()
	}
}

var b bool
