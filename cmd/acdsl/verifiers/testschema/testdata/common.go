package fixture

type commonT struct{}

func (t *commonT) Run(name string, f func(t *commonT)) {}

type commonCase struct {
	name string
}

func commonStyle(t *commonT) {
	tests := []commonCase{{name: "ok"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *commonT) {})
	}
	t.Run("ad-hoc subtest", func(t *commonT) {})
}
