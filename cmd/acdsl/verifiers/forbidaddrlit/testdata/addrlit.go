package fixture

type config struct {
	timeout int
}

func addrLit() *config {
	return &config{timeout: 5}
}
