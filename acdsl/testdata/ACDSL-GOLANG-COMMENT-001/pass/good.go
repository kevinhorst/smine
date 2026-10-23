package fixture

// describe names the fixture.
func describe() string {
	return "ok"
}

// load returns its path unchanged.
//nolint:unparam // fixture only
func load(path string) string {
	return path
}
