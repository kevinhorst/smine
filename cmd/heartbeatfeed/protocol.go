// cmd/heartbeatfeed/protocol.go — the protocol facts of this hook; the one file the literal gate exempts.

package main

import "time"

const (
	addrEnv     = "HEARTBEAT_ADDR"
	bodyCap     = 1 << 20
	contentType = "application/json"
	exitOk      = 0
	hookPath    = "/hook"
	postTimeout = time.Second
)
