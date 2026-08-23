//go:build !windows

package main

import "fmt"

// run keeps `go build ./...` green on non-windows platforms; launchd invokes
// run.sh directly there and this binary has no job.
func run(dir string) int {
	fmt.Printf("routinewrap is windows-only; %s is run by launchd via run.sh\n", dir)
	return 2
}
