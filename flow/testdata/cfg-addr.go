package p

func f() {
	var ss serializedFileSet
	if err := decode(&ss); err != nil {
		return err
	}

	s.mutex.Lock()
	s.base = ss.Base
}
