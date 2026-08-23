package fixture

type testingT struct{}

func (t *testingT) Run(name string, f func(t *testingT)) {}

type testCase struct {
	_id string
}

func tableTest(t *testingT) {
	tests := []*testCase{{_id: "pass-ok"}}
	for _, test := range tests {
		t.Run(test._id, func(t *testingT) {})
	}
}
