//go:build !windows

package main

import (
	"context"
	"fmt"
)

// runInstall exists only on Windows (plan D18) — install.sh is the macOS
// path and two installers for one platform would drift.
func runInstall(ctx context.Context, addr string, initWelcome bool, peekPort, peekControlPort int) int {
	_ = ctx
	fmt.Printf("-install is windows-only; use ./install.sh (addr %s, peek %d/%d)\n",
		addr, peekPort, peekControlPort)
	return 2
}
