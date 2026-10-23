package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// demoValueLength is the hex length of a heartbeat-demo producer value.
const demoValueLength = 12

// demoClosedFile is the marker demo-close writes into the run's dir.
const demoClosedFile = "closed.json"

func runDemoValue(args []string) int {
	flags := flag.NewFlagSet("demo-value", flag.ExitOnError)
	seed := flags.String("seed", "", "run seed; required")
	salt := flags.String("salt", "run", "value salt")
	flags.Parse(args)
	if *seed == "" {
		fmt.Fprintln(os.Stderr, "usage: rules demo-value --seed <s> [--salt <x>]")
		return exitError
	}

	return printJson(map[string]string{"value": demoValue(*seed, *salt)})
}

func runDemoClose(args []string) int {
	flags := flag.NewFlagSet("demo-close", flag.ExitOnError)
	dir := flags.String("dir", "", "the run's artifacts dir; required")
	flags.Parse(args)
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: rules demo-close --dir <d>")
		return exitError
	}

	// a denied stage (hand-spawn, plan-mode) never created the dir; the close still lands
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	if err := os.WriteFile(filepath.Join(*dir, demoClosedFile), []byte("{\"closed\": true}\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printJson(map[string]bool{"closed": true})
}

func demoValue(seed string, salt string) string {
	sum := sha256.Sum256([]byte(seed + "/" + salt))
	return hex.EncodeToString(sum[:])[:demoValueLength]
}
