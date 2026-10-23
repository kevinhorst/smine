package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// claimedInterfaces is what this build implements; TestProviderClaims pins it to skills/interfaces.cue.
var claimedInterfaces = []string{
	"routine-publish-style@1",
	"routine-worker@1",
}

// interfacesAnswer is the handshake document every provider prints.
type interfacesAnswer struct {
	Interfaces []string `json:"interfaces"`
}

func printInterfaces(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	answer := &interfacesAnswer{Interfaces: claimedInterfaces}
	data, err := json.Marshal(answer)
	if err != nil {
		fmt.Fprintln(os.Stderr, "routine:", err)
		return exitConfig
	}

	fmt.Fprintln(os.Stdout, string(data))
	return 0
}
