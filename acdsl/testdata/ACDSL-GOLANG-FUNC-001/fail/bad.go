package fixture

import "context"

func ctxSecond(path string, ctx context.Context) error {
	_ = ctx
	_ = path
	return nil
}

func errorNotLast(c context.Context) (error, string) {
	_ = c
	return nil, ""
}

func fourReturns() (int, string, bool, error) {
	return 0, "", false, nil
}
