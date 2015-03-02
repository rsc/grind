package p

type T map[int]int
type X [10]int
type S struct {
	X, Y, Z int
}

func f() {
	x := (*T)(nil)
	y := X{}
	z := [12]int{}
	w := struct{ X, Y int }{}
	a := S{}
	b := struct{ X, Y int }{1, 2}
	c := (map[string]int)(nil)
	d := (chan int)(nil)

	_ = x
	_ = y
	_ = z
	_ = w
	_ = a
	_ = b
	_ = c
	_ = d
}
