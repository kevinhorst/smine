// Package xp0 is the ACDSL experiment tree's seeded package: governed by the
// xp-anchored rules, deliberately free of pointer initialization and
// subprocess usage, so a probe reading it receives rules only through the
// projection channel — never through an in-file exemplar.
package xp0

// SeedLimit is a placeholder constant so the package has content to read.
const SeedLimit = 8

// Describe returns a fixed description string.
func Describe() string {
	return "xp0 experiment seed"
}
