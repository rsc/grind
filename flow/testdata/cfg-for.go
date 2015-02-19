package p

func f() {
L:
	for pre(); cond(); post() {
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
}
