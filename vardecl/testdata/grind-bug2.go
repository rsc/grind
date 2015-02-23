package p

func f() int {
	var x int
	fail := func() int { return 1 }
	for {
		x++
		if x == 100 {
			break
		}
		if x == 99 {
			return fail()
		}
	}
	return 2
}
