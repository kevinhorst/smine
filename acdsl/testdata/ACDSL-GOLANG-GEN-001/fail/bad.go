// Package main is a bounds-violating fixture: it imports a module that is
// not in go.mod's require set — a new dependency the gate must refuse.
package main

import (
	"fmt"

	"github.com/fabricated/notadep"
)

func main() {
	fmt.Println(notadep.Version)
}
