package p

func f() {
	var i int

	for i = 0; i < 10; i++ {
		use(i)
	}

	for i = 0; i < 10; i++ {
		use(i)
	}

	var j int
	for j += 1; j < 10; j++ {
	}
}

func use(interface{})
