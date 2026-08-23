// Package main is a compliant verifier fixture: stdlib + module-internal
// imports only, well under the line bound.
package main

import (
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/shell"
)

var _ = shell.Timeout

func main() {
	fmt.Fprintln(os.Stdout, "ok")
}
