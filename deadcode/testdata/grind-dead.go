package p

var b bool

func f(x int) {
	if b {
		if b {
			return
		}
		if b {
			goto Y
		}
		return
		goto X
		goto Z
	}
	return

X:
	f(1)
Z:
	f(2)
Y:
	f(3)
}
