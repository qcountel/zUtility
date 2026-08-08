package misc

func CallMethods(x []func()) {
	for _, fn := range x {
		fn()
	}
}
