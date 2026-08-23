package fixture

import "context"

func load(ctx context.Context, path string) (string, error) {
	_ = ctx
	return path, nil
}

func describe() string {
	return "ok"
}
