package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// claimedInterfaces is what this build implements; TestProviderClaims pins it to skills/interfaces.cue.
var claimedInterfaces = []string{
	"base@1",
	"buildtest@1",
	"capture-ui@1",
	"cleanup@1",
	"demo-close@1",
	"demo-value@1",
	"entries@1",
	"generate@1",
	"intake@1",
	"lanes@1",
	"ledger@1",
	"merge-branch@1",
	"pairs@1",
	"plan-claims@1",
	"plan-display@1",
	"plan-sections@1",
	"present@1",
	"probe-replay@1",
	"provision@1",
	"prune@1",
	"reset-round@1",
	"snapshot@1",
	"store-root@1",
	"task-check@1",
	"title-set@1",
}

// interfacesAnswer is the handshake document every provider prints.
type interfacesAnswer struct {
	Interfaces []string `json:"interfaces"`
}

func runInterfaces(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: rules interfaces")
		return exitError
	}

	answer := &interfacesAnswer{Interfaces: claimedInterfaces}
	data, err := json.Marshal(answer)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	fmt.Println(string(data))
	return exitClean
}
