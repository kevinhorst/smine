// cmd/playsound/protocol.go — the protocol facts of this hook.

package main

import "time"

const (
	afplayBinary = "/usr/bin/afplay"
	exitOk       = 0
	headlessEnv  = "SMINE_HEADLESS"
	playTimeout  = 10 * time.Second
)

// PlaySoundW flags: the argument is a file name, the call blocks until the sound ends, a missing file plays nothing.
const (
	sndFilename  = 0x00020000
	sndNoDefault = 0x00000002
	sndSync      = 0x00000000
)
