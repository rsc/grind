package p

func f() {
	before()
L:
	for k, v = range expr {
		body()
		if brk() {
			break
		}
		if cont() {
			continue
		}
		if brkL() {
			break L
		}
		if contL() {
			continue L
		}
		more()
	}
	after()
}
