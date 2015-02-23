package p

func f() {
	var x int
	var z int
	{
		switch z {
		case 1:
			use(&x)
			use(x)
			use(x)
		case 2:
			use(&x)
			use(x)
			use(x)
		}
	}
}

func use(interface{})
