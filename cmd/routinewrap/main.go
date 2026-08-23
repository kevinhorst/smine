// routinewrap is the Windows Task Scheduler action for routines: it reads the
// routine plist, injects its env, takes the self- and group locks, arms the
// backstop deadline, holds a wake-lock, and runs run.sh under Git Bash.
// macOS launchd invokes run.sh directly and never uses this binary.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: routinewrap <routine-dir>")
		os.Exit(2)
	}
	os.Exit(run(os.Args[1]))
}
