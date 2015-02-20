package p

func f() {
	{
		var i int

		x()
		i++
		if b {
		L1:
			goto L1
		}
		use(i)
	}
	{
		var i int

	L2:
		x()
		i++
		if b {
			goto L2
		}
		use(i)
	}

	{
		var i int

		if b {
			goto L3
		}
		i = 10
		use(i)
	L3:
	}
	{
		{
			var i int

			if b {
				goto L4
			}
			i = 10
			use(i)
		}
	L4:
	}

}
