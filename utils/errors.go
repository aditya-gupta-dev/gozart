package utils

func Unwrap[T any](ok T, err error) T {
	if err != nil {
		panic(err)
	}
	return ok
}

func Try[T any](err error) {
	if err != nil {
		panic(err)
	}
}
