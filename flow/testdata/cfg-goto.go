package p

func f() {
	top()
L1:
	l1()
	if gotoL1() {
		goto L1
	}
	if gotoL2() {
		goto L2
	}
	beforeL2()
L2:
	l2()
	if gotoL1x() {
		goto L1
	}
}
