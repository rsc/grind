package p

func f1() int {
	c := 1
	{
		c := 2
		use(c)
		goto ret
	}
	return 0

ret:
	use(c)
	return 1
}

func f2() (c int) {
	c = 1
	{
		c := 2
		use(c)
		goto ret
	}
	return 0

ret:
	return
}

func f3() (c int) {
	c = 1
	{
		c := 2
		use(c)
		goto ret
	}
	return 0

ret:
	return c
}

func use(interface{})
