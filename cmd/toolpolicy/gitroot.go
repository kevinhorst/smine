package main

import "github.com/kevinhorst/smine/internal/gitroot"

func isLinkedWorktree(cwd string) bool {
	return gitroot.MainRoot(cwd) != gitroot.CheckoutRoot(cwd)
}
