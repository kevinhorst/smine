package main

const increment = 42

func main() {
	retries := 0
	retries = grow(retries)
	_ = retries
}

func grow(in int) int {
	return in + increment
}
