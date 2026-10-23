package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckHeadings(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		body string
	}

	tests := make([]*testCase, 0)

	// catalog-in-order
	tests = append(tests, &testCase{
		_id:         "catalog-in-order",
		_violations: 0,

		body: "## TLDR\n<x>\n\n## Scope\n<x>\n\n## Changes\n<x>\n\n## Current state [AGENT-ONLY]\n<x>\n\n## Changelog\n<x>\n",
	})

	// tldr-not-first
	tests = append(tests, &testCase{
		_id:         "tldr-not-first",
		_violations: 2,

		body: "## Scope\n<x>\n\n## TLDR\n<x>\n",
	})

	// out-of-order
	tests = append(tests, &testCase{
		_id:         "out-of-order",
		_violations: 1,

		body: "## TLDR\n<x>\n\n## Changes\n<x>\n\n## Scope\n<x>\n",
	})

	// override-marked
	tests = append(tests, &testCase{
		_id:         "override-marked",
		_violations: 0,

		body: "## TLDR\n<x>\n\n## Root cause\n<override: a walk has no catalog section>\n",
	})

	// override-missing
	tests = append(tests, &testCase{
		_id:         "override-missing",
		_violations: 1,

		body: "## TLDR\n<x>\n\n## Root cause\n<a walk>\n",
	})

	// agent-marker-missing
	tests = append(tests, &testCase{
		_id:         "agent-marker-missing",
		_violations: 1,

		body: "## TLDR\n<x>\n\n## Current state\n<x>\n",
	})

	// marker-on-override
	tests = append(tests, &testCase{
		_id:         "marker-on-override",
		_violations: 0,

		body: "## TLDR\n<x>\n\n## Ruled out [AGENT-ONLY]\n<override: a refuted register has no catalog section>\n",
	})

	// marker-on-human-section
	tests = append(tests, &testCase{
		_id:         "marker-on-human-section",
		_violations: 1,

		body: "## TLDR\n<x>\n\n## Scope [AGENT-ONLY]\n<x>\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			out := &bytes.Buffer{}
			lines := strings.Split(test.body, "\n")
			assert.Equal(t, test._violations, checkHeadings("payloads/SKILL-X-TPL-001.md", templateHeadings(lines), lines, out), out.String())
		})
	}
}
