// Command playsound plays the sound file a hook group names: afplay on macOS, PlaySound on Windows, nothing elsewhere.
package main

import (
	"context"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run never fails the hook: no argument, a headless cell or a player error all exit 0, as the shell script did.
func run(args []string) int {
	if len(args) == 0 || os.Getenv(headlessEnv) != "" {
		return exitOk
	}

	ctx, cancel := context.WithTimeout(context.Background(), playTimeout)
	defer cancel()
	_ = play(ctx, args[0])
	return exitOk
}
