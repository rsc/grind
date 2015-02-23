package p

type T struct {
	X     int
	Array [4]int
	Slice []int
	Ptr   *T
}

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
		var i T
		{
			i = T{X: 1}
			_ = &i.Array
		}
		{
			i = T{X: 2}
			use(i)
		}
	}
	{
		var i T
		{
			i = T{X: 1}
			_ = &i.Slice
		}
		{
			i = T{X: 2}
			use(i)
		}
	}
	{
		var i T
		{
			i = T{X: 1}
			_ = &i.Slice[0]
		}
		{
			i = T{X: 2}
			use(i)
		}
	}
	{
		var i [2]int
		{
			i = [2]int{1, 2}
			_ = &i[0]
		}
		{
			i = [2]int{3, 4}
			use(i)
		}
	}
	{
		var i []int
		{
			i = []int{1, 2}
			_ = &i[0]
		}
		{
			i = []int{3, 4}
			use(i)
		}
	}
	{
		var i T
		{
			i = T{X: 3}
			_ = &i.Array[0]
		}
		{
			i = T{X: 4}
			use(i)
		}
	}
	{
		var i T
		{
			i = T{X: 3}
			_ = &i.Slice[0]
		}
		{
			i = T{X: 4}
			use(i)
		}
	}
	{
		var i [2]T
		{
			i = [2]T{{X: 5}, {X: 6}}
			_ = &i[0].X
		}
		{
			i = [2]T{{X: 7}, {X: 8}}
			use(i)
		}
	}
	{
		var i [2]T
		{
			i = [2]T{{X: 5}, {X: 6}}
			_ = &i[0].Ptr.X
		}
		{
			i = [2]T{{X: 7}, {X: 8}}
			use(i)
		}
	}
	{
		var i [2]T
		{
			i = [2]T{{X: 5}, {X: 6}}
			_ = (&(((i)[0]).X))
		}
		{
			i = [2]T{{X: 7}, {X: 8}}
			use(i)
		}
	}
}

func use(interface{})
