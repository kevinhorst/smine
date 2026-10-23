package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// claimedInterfaces is what this build implements; TestProviderClaims pins it to skills/interfaces.cue.
var claimedInterfaces = []string{
	"acdsl-check@1",
	"acdsl-fixtures@1",
	"lint@1",
	"project-plan@1",
	"project-strip@1",
}

// interfacesAnswer is the handshake document every provider prints.
type interfacesAnswer struct {
	Interfaces []string `json:"interfaces"`
}

func runInterfaces(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: acdsl interfaces")
		return exitError
	}

	answer := &interfacesAnswer{Interfaces: claimedInterfaces}
	data, err := json.Marshal(answer)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acdsl:", err)
		return exitError
	}

	fmt.Println(string(data))
	return exitClean
}
