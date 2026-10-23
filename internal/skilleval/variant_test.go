package skilleval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
)

func TestParseVariant(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    *Variant
		_expectedErr error

		descriptor string
	}

	tests := make([]*testCase, 0)

	// model-short-name
	tests = append(tests, &testCase{
		_id:          "model-short-name",
		_expected:    &Variant{Descriptor: "model:sonnet", Factor: factorModel, Value: "sonnet"},
		_expectedErr: nil,

		descriptor: "model:sonnet",
	})

	// model-full-id
	tests = append(tests, &testCase{
		_id:          "model-full-id",
		_expected:    &Variant{Descriptor: "model:claude-opus-5-5[1m]", Factor: factorModel, Value: "claude-opus-5-5[1m]"},
		_expectedErr: nil,

		descriptor: "model:claude-opus-5-5[1m]",
	})

	// effort-low
	tests = append(tests, &testCase{
		_id:          "effort-low",
		_expected:    &Variant{Descriptor: "effort:low", Factor: factorEffort, Value: "low"},
		_expectedErr: nil,

		descriptor: "effort:low",
	})

	// no-goland
	tests = append(tests, &testCase{
		_id:          "no-goland",
		_expected:    &Variant{Descriptor: "no-goland", Factor: factorNo, Value: "goland"},
		_expectedErr: nil,

		descriptor: "no-goland",
	})

	// overlay-relative
	tests = append(tests, &testCase{
		_id:          "overlay-relative",
		_expected:    &Variant{Descriptor: "overlay:x/y.patch", Factor: factorOverlay, Value: "x/y.patch"},
		_expectedErr: nil,

		descriptor: "overlay:x/y.patch",
	})

	// version-range
	tests = append(tests, &testCase{
		_id:          "version-range",
		_expected:    &Variant{Descriptor: "version:1.0.0..1.1.0", Factor: factorVersion, Value: "1.0.0..1.1.0"},
		_expectedErr: nil,

		descriptor: "version:1.0.0..1.1.0",
	})

	// model-empty
	tests = append(tests, &testCase{
		_id:          "model-empty",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "model:",
	})

	// model-with-effort
	tests = append(tests, &testCase{
		_id:          "model-with-effort",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "model:opus:high",
	})

	// effort-unknown
	tests = append(tests, &testCase{
		_id:          "effort-unknown",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "effort:huge",
	})

	// no-empty
	tests = append(tests, &testCase{
		_id:          "no-empty",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "no-",
	})

	// no-with-colon
	tests = append(tests, &testCase{
		_id:          "no-with-colon",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "no-a:b",
	})

	// overlay-parent
	tests = append(tests, &testCase{
		_id:          "overlay-parent",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "overlay:../x",
	})

	// overlay-absolute
	tests = append(tests, &testCase{
		_id:          "overlay-absolute",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "overlay:/abs",
	})

	// version-single
	tests = append(tests, &testCase{
		_id:          "version-single",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "version:1.0.0",
	})

	// version-equal
	tests = append(tests, &testCase{
		_id:          "version-equal",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "version:1.0.0..1.0.0",
	})

	// version-open-start
	tests = append(tests, &testCase{
		_id:          "version-open-start",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "version:..1",
	})

	// unknown-factor
	tests = append(tests, &testCase{
		_id:          "unknown-factor",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "temperature:1",
	})

	// empty
	tests = append(tests, &testCase{
		_id:          "empty",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "",
	})

	// two-changes-space
	tests = append(tests, &testCase{
		_id:          "two-changes-space",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "model:sonnet effort:low",
	})

	// two-changes-plus
	tests = append(tests, &testCase{
		_id:          "two-changes-plus",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "model:sonnet+no-goland",
	})

	// two-changes-comma
	tests = append(tests, &testCase{
		_id:          "two-changes-comma",
		_expected:    nil,
		_expectedErr: ErrBadVariant,

		descriptor: "model:sonnet,opus",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			variant, err := ParseVariant(test.descriptor)
			require.ErrorIs(t, err, test._expectedErr)
			assert.Equal(t, test._expected, variant)
		})
	}
}

func TestVariant_apply(t *testing.T) {
	type testCase struct {
		_id           string
		_expected     *armConfig
		_expectedBase *armConfig
		_expectedErr  error

		base    *armConfig
		variant *Variant
	}

	baseline := newBaseConfig(&evals.Definition{})
	tests := make([]*testCase, 0)

	// model-change
	tests = append(tests, &testCase{
		_id:           "model-change",
		_expected:     &armConfig{disabledServers: []string{}, effort: "high", key: "sonnet-high", model: "sonnet"},
		_expectedBase: baseline,
		_expectedErr:  nil,

		base:    newBaseConfig(&evals.Definition{}),
		variant: &Variant{Descriptor: "model:sonnet", Factor: factorModel, Value: "sonnet"},
	})

	// model-equal-to-baseline
	tests = append(tests, &testCase{
		_id:           "model-equal-to-baseline",
		_expected:     nil,
		_expectedBase: baseline,
		_expectedErr:  ErrBadVariant,

		base:    newBaseConfig(&evals.Definition{}),
		variant: &Variant{Descriptor: "model:opus", Factor: factorModel, Value: "opus"},
	})

	// effort-change
	tests = append(tests, &testCase{
		_id:           "effort-change",
		_expected:     &armConfig{disabledServers: []string{}, effort: "low", key: "opus-low", model: "opus"},
		_expectedBase: baseline,
		_expectedErr:  nil,

		base:    newBaseConfig(&evals.Definition{}),
		variant: &Variant{Descriptor: "effort:low", Factor: factorEffort, Value: "low"},
	})

	// no-goland
	tests = append(tests, &testCase{
		_id:           "no-goland",
		_expected:     &armConfig{disabledServers: []string{"goland"}, effort: "high", key: "opus-high-no-goland", model: "opus"},
		_expectedBase: baseline,
		_expectedErr:  nil,

		base:    newBaseConfig(&evals.Definition{}),
		variant: &Variant{Descriptor: "no-goland", Factor: factorNo, Value: "goland"},
	})

	// overlay
	tests = append(tests, &testCase{
		_id:           "overlay",
		_expected:     &armConfig{disabledServers: []string{}, effort: "high", key: "opus-high-overlay", model: "opus", variantPatch: "x/y.patch"},
		_expectedBase: baseline,
		_expectedErr:  nil,

		base:    newBaseConfig(&evals.Definition{}),
		variant: &Variant{Descriptor: "overlay:x/y.patch", Factor: factorOverlay, Value: "x/y.patch"},
	})

	// version
	tests = append(tests, &testCase{
		_id:           "version",
		_expected:     nil,
		_expectedBase: baseline,
		_expectedErr:  ErrBadVariant,

		base:    newBaseConfig(&evals.Definition{}),
		variant: &Variant{Descriptor: "version:1.0.0..1.1.0", Factor: factorVersion, Value: "1.0.0..1.1.0"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			varied, err := test.variant.apply(test.base)
			require.ErrorIs(t, err, test._expectedErr)
			assert.Equal(t, test._expected, varied)
			assert.Equal(t, test._expectedBase, test.base)
		})
	}

	// baseline-slice-not-aliased: two variants of one baseline with spare
	// capacity keep their own disabled lists
	shared := newBaseConfig(&evals.Definition{})
	shared.disabledServers = make([]string, 0, 2)
	first, err := (&Variant{Descriptor: "no-goland", Factor: factorNo, Value: "goland"}).apply(shared)
	require.NoError(t, err)
	second, err := (&Variant{Descriptor: "no-github", Factor: factorNo, Value: "github"}).apply(shared)
	require.NoError(t, err)
	assert.Equal(t, []string{"goland"}, first.disabledServers)
	assert.Equal(t, []string{"github"}, second.disabledServers)
	assert.Empty(t, shared.disabledServers)
}
