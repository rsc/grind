package p

type T struct {
	X int
}

func decode(*T) error

func f() error {
	var t T
	if err := decode(&t); err != nil {
		return err
	}
	print(t.X)
	return nil
}
