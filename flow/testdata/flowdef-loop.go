package p

func f() {
	var i int
	i++

	for i = 0; i < 10; i++ {
		use(i)
	}

	for i = 0; i < 10; i++ {
		use(i)
	}

	use(i)
}
