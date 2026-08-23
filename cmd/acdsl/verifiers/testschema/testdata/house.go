package fixture

type testingT struct{}

func (t *testingT) Run(name string, f func(t *testingT)) {}

type houseCase struct {
	_id string
}

func houseStyle(t *testingT) {
	tests := []*houseCase{{_id: "pass-ok"}}
	for _, test := range tests {
		t.Run(test._id, func(t *testingT) {})
	}
}
