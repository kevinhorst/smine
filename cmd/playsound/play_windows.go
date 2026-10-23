//go:build windows

package main

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var playSound = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

func play(ctx context.Context, path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("play: %w", err)
	}
	_, _, _ = playSound.Call(uintptr(unsafe.Pointer(name)), 0, sndFilename|sndSync|sndNoDefault)
	return nil
}
