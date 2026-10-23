package hook

const inlineCap = 9000

func isOverLimit(n int) bool {
	if n > 50 {
		return true
	}
	return n > inlineCap
}
