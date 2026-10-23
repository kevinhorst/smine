// Package toolpolicy names the tool-policy section the gate reads from the hooks config.
package toolpolicy

import "github.com/kevinhorst/smine/internal/hookconfig"

const (
	ClassBash        = hookconfig.ClassBash
	ClassRead        = hookconfig.ClassRead
	ClassWrite       = hookconfig.ClassWrite
	ClassRuleBearing = hookconfig.ClassRuleBearing
	ClassUnknown     = hookconfig.ClassUnknown
)

type Policy = hookconfig.Toolpolicy
