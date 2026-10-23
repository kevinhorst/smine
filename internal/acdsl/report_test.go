package acdsl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildReportGroupsByRule(t *testing.T) {
	registry := map[string]RegistryEntry{
		"gofmt": {Description: "run gofmt -w"},
	}
	diagnostics := []Diagnostic{
		{RuleId: "ACDSL-B-001", Verifier: "vet", Message: "b.go:3: bad", Why: "b doctrine"},
		{RuleId: "ACDSL-A-001", Verifier: "gofmt", Message: "a.go:1: unformatted", Why: "a doctrine"},
		{RuleId: "ACDSL-A-001", Verifier: "gofmt", Message: "c.go:9: unformatted", Why: "a doctrine"},
	}

	report := BuildReport(registry, diagnostics)

	assert.Contains(t, report, "### ACDSL-A-001 — gofmt")
	assert.Contains(t, report, "a doctrine")
	assert.Contains(t, report, "- a.go:1: unformatted")
	assert.Contains(t, report, "- c.go:9: unformatted")
	assert.Contains(t, report, "fix: run gofmt -w")
	assert.Contains(t, report, "### ACDSL-B-001 — vet")
	assert.Contains(t, report, "3 violation(s) · 2 rule(s) · 3 file(s)")
	assert.Less(t, strings.Index(report, "ACDSL-A-001"), strings.Index(report, "ACDSL-B-001"))
}

func TestBuildReportEmptyDiagnostics(t *testing.T) {
	report := BuildReport(map[string]RegistryEntry{}, nil)

	assert.Empty(t, report)
}

func TestBuildReportUnknownVerifierNoFixHint(t *testing.T) {
	diagnostics := []Diagnostic{
		{RuleId: "ACDSL-A-001", Verifier: "ghost", Message: "a.go:1: bad", Why: "doctrine"},
	}

	report := BuildReport(map[string]RegistryEntry{}, diagnostics)

	assert.NotContains(t, report, "fix:")
}
