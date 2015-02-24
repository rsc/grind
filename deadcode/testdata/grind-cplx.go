package p

func dead()

func f(x int) {
	for {
		switch x {
		case 1:
			break
			dead()
		case 2:
			continue
			dead()
		}
	}
	dead()
}
