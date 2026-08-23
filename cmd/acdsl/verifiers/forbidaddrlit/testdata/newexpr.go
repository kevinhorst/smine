package fixture

type conf struct {
	timeout int
}

func newExpr() *conf {
	return new(conf{timeout: 5})
}

func addrOfVar() *conf {
	value := conf{timeout: 7}
	return &value
}
