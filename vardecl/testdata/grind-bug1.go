package p

var y int

func g() bool

func f() int {
	var x int
	for g() {
		x = y
	}
	return x
}
