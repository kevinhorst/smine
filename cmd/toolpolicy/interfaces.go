package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// interfacesAnswer is the handshake document every provider prints.
type interfacesAnswer struct {
	Interfaces []string `json:"interfaces"`
}

func runInterfaces(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: toolpolicy interfaces")
		return usageExitCode
	}

	answer := &interfacesAnswer{Interfaces: claimedInterfaces}
	data, err := json.Marshal(answer)
	if err != nil {
		fmt.Fprintf(stderr, "runInterfaces: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, string(data))
	return 0
}
