package fixture

// oneLine carries a one-line comment.
func oneLine() {}

// threeLines carries a block
// that runs past the cap
// by two lines.
func threeLines() {}

//go:noinline
//nolint:unused // fixture only
func directivesOnly() {}

func directiveUnderComment() {
	// the directive line below is not counted.
	//nolint:ineffassign // fixture only
	_ = 0
}

func blockComment() {
	/* a block comment spans
	two lines */
}

// docOverDirective keeps the bare separator line gofmt inserts before a directive.
//
//go:noinline
func docOverDirective() {}
