package p

var cond bool

func oploop() {
	var a1 int
	a1 = int(1)
	if cond {
		a1 = f()
		g = int8(a1)
	}
	a1--
	_ = x[a1]
}

func f() int

var g int8
var x []int
