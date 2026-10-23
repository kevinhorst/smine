package hook

type settings struct {
	Cap int
}

func isOverCap(config *settings, count int) bool {
	return count > config.Cap
}
