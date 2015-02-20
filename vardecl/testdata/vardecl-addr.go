package p

func f() {
	{
		var i int

		{
			i = 10
			use(i)
		}
		{
			i = 11
			use(i)
		}
	}
	{
		var i int

		{
			i = 10
			_ = &i
		}
		{
			i = 11
			use(i)
		}
	}
	{
		var i int
		{
			i = 10
			_ = &i.x
		}
		{
			i = 11
			use(i)
		}
	}
	{
		var i int
		{
			i = 10
			_ = &i[0]
		}
		{
			i = 11
			use(i)
		}
	}
	{
		var i int
		{
			i = 10
			_ = &i.x[0]
		}
		{
			i = 11
			use(i)
		}
	}
	{
		var i int
		{
			i = 10
			_ = &i[0].x
		}
		{
			i = 11
			use(i)
		}
	}
	{
		var i int
		{
			i = 10
			_ = (&(((i)[0]).x))
		}
		{
			i = 11
			use(i)
		}
	}
}
