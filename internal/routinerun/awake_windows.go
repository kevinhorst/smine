package routinerun

import "golang.org/x/sys/windows"

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// PreventSleep is the caffeinate -is analog; WakeToRun in the task XML is the wake-from-sleep backstop.
func PreventSleep() func() {
	setThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired))
	return func() { setThreadExecutionState.Call(uintptr(esContinuous)) }
}
