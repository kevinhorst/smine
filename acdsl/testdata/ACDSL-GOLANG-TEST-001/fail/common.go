package fixture

type testingT struct{}

func (t *testingT) Run(name string, f func(t *testingT)) {}

type testCase struct {
	name string
}

func tableTest(t *testingT) {
	tests := []testCase{{name: "ok"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testingT) {})
	}
}
