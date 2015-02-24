package p

func f(b bool) {
	var x [1]string
	var i int

	if b {
		for {
			x[0] = "xxx"
			break
		}
	}

	for i = 0; i < 10; i++ {
		if x[i] == "yyy" {
			break
		}
	}
}
