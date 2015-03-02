package p

func test(s string) []string {
	var p []string
	switch s {
	case "test":
		p = append(p, "hello")
	}
	return p
}
