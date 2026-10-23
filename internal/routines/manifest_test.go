package routines

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		content string
	}

	tests := make([]*testCase, 0)

	// fail-missing-file
	tests = append(tests, &testCase{
		_id:         "fail-missing-file",
		_shouldPass: false,

		content: "",
	})

	// fail-invalid-json
	tests = append(tests, &testCase{
		_id:         "fail-invalid-json",
		_shouldPass: false,

		content: `{"kind":`,
	})

	// fail-unknown-kind
	tests = append(tests, &testCase{
		_id:         "fail-unknown-kind",
		_shouldPass: false,

		content: `{"kind":"weekly","prompt":"/x"}`,
	})

	// fail-single-without-prompt
	tests = append(tests, &testCase{
		_id:         "fail-single-without-prompt",
		_shouldPass: false,

		content: `{"kind":"single"}`,
	})

	// fail-retired-skill-eval-kind
	tests = append(tests, &testCase{
		_id:         "fail-retired-skill-eval-kind",
		_shouldPass: false,

		content: `{"kind":"skill-eval","evalSkill":"railroad-review"}`,
	})

	// fail-budget-with-prompt
	tests = append(tests, &testCase{
		_id:         "fail-budget-with-prompt",
		_shouldPass: false,

		content: `{"kind":"budget","prompt":"/smine --nightly"}`,
	})

	// pass-budget-without-prompt
	tests = append(tests, &testCase{
		_id:         "pass-budget-without-prompt",
		_shouldPass: true,

		content: `{"kind":"budget"}`,
	})

	// pass-single
	tests = append(tests, &testCase{
		_id:         "pass-single",
		_shouldPass: true,

		content: `{"kind":"single","prompt":"/coverage-increase --nightly"}`,
	})

	// pass-smine-nightly
	tests = append(tests, &testCase{
		_id:         "pass-smine-nightly",
		_shouldPass: true,

		content: `{"kind":"smine-nightly","prompt":"/smine --nightly"}`,
	})

	// fail-group-uppercase
	tests = append(tests, &testCase{
		_id:         "fail-group-uppercase",
		_shouldPass: false,

		content: `{"kind":"single","prompt":"/x","group":"Smine"}`,
	})

	// pass-group
	tests = append(tests, &testCase{
		_id:         "pass-group",
		_shouldPass: true,

		content: `{"kind":"single","prompt":"/x","group":"skill-eval"}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dir := t.TempDir()
			if test.content != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFileName), []byte(test.content), 0o644))
			}

			_, err := LoadManifest(dir)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestManifest_Skill(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		prompt string
	}

	tests := make([]*testCase, 0)

	// slash-skill-with-args
	tests = append(tests, &testCase{
		_id:       "slash-skill-with-args",
		_expected: "smine",

		prompt: "/smine --nightly",
	})

	// placeholder-prompt
	tests = append(tests, &testCase{
		_id:       "placeholder-prompt",
		_expected: "",

		prompt: "<the claude -p prompt>",
	})

	// no-prompt
	tests = append(tests, &testCase{
		_id:       "no-prompt",
		_expected: "",

		prompt: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			manifest := &Manifest{Kind: KindSingle, Prompt: test.prompt}

			assert.Equal(t, test._expected, manifest.Skill())
		})
	}
}
