package main

func main() {
	retries := 0
	retries = magic(retries)
	_ = retries
}

func magic(in int) int {
	return in + 42
}
