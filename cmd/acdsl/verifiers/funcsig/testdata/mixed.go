package fixture

import "context"

func good(ctx context.Context, path string) (string, error) {
	_ = ctx
	return path, nil
}

func noContext() string {
	return "ok"
}

type holder struct{}

func (h *holder) method(ctx context.Context) error {
	_ = ctx
	return nil
}

func ctxSecond(path string, ctx context.Context) error {
	_ = ctx
	_ = path
	return nil
}

func ctxMisnamed(c context.Context) error {
	_ = c
	return nil
}

func errorNotLast(ctx context.Context) (error, string) {
	_ = ctx
	return nil, ""
}

func fourReturns() (int, string, bool, error) {
	return 0, "", false, nil
}
