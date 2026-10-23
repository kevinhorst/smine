package budget

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPair_Agreement(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Agreement

		pair      *Pair
		tolerance Tolerance
	}

	tests := make([]*testCase, 0)

	// both-zero-agree
	bothZero := &Pair{}
	bothZero.Add(0, true, true, 0)
	tests = append(tests, &testCase{
		_id:       "both-zero-agree",
		_expected: AgreementAgree,

		pair:      bothZero,
		tolerance: TokenTolerance,
	})

	// within-relative
	withinRelative := &Pair{}
	withinRelative.Add(100, true, true, 100.5)
	tests = append(tests, &testCase{
		_id:       "within-relative",
		_expected: AgreementAgree,

		pair:      withinRelative,
		tolerance: TokenTolerance,
	})

	// beyond-relative
	beyondRelative := &Pair{}
	beyondRelative.Add(100, true, true, 110)
	tests = append(tests, &testCase{
		_id:       "beyond-relative",
		_expected: AgreementDiffer,

		pair:      beyondRelative,
		tolerance: CostTolerance,
	})

	// within-absolute
	withinAbsolute := &Pair{}
	withinAbsolute.Add(1000, true, true, 1060)
	tests = append(tests, &testCase{
		_id:       "within-absolute",
		_expected: AgreementAgree,

		pair:      withinAbsolute,
		tolerance: WallTolerance,
	})

	// beyond-absolute
	beyondAbsolute := &Pair{}
	beyondAbsolute.Add(1000, true, true, 1061)
	tests = append(tests, &testCase{
		_id:       "beyond-absolute",
		_expected: AgreementDiffer,

		pair:      beyondAbsolute,
		tolerance: WallTolerance,
	})

	// single-only
	singleOnly := &Pair{}
	singleOnly.Add(5, true, false, 0)
	singleOnly.Add(0, false, true, 7)
	tests = append(tests, &testCase{
		_id:       "single-only",
		_expected: AgreementSingle,

		pair:      singleOnly,
		tolerance: TokenTolerance,
	})

	// mixed-single-and-both-uses-paired
	mixed := &Pair{}
	mixed.Add(100, true, true, 100)
	mixed.Add(500, true, false, 0)
	tests = append(tests, &testCase{
		_id:       "mixed-single-and-both-uses-paired",
		_expected: AgreementAgree,

		pair:      mixed,
		tolerance: CostTolerance,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.pair.Agreement(test.tolerance))
		})
	}
}

func TestPair_AddAndMerge(t *testing.T) {
	session := &Pair{}
	session.Add(100, true, true, 90)
	other := &Pair{}
	other.Add(5, true, false, 0)
	other.Add(0, false, true, 7)
	other.Add(0, false, false, 0)

	session.Merge(other)

	expected := &Pair{Both: 1, Ccusage: 105, PairedCcusage: 100, PairedPeek: 90, Peek: 97, Single: 2}
	assert.Equal(t, expected, session)
}

func TestWorst(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Agreement

		verdicts []Agreement
	}

	tests := make([]*testCase, 0)

	// any-differ
	tests = append(tests, &testCase{
		_id:       "any-differ",
		_expected: AgreementDiffer,

		verdicts: []Agreement{AgreementAgree, AgreementSingle, AgreementDiffer, AgreementAgree},
	})

	// agree-and-single
	tests = append(tests, &testCase{
		_id:       "agree-and-single",
		_expected: AgreementAgree,

		verdicts: []Agreement{AgreementSingle, AgreementAgree, AgreementSingle},
	})

	// all-single
	tests = append(tests, &testCase{
		_id:       "all-single",
		_expected: AgreementSingle,

		verdicts: []Agreement{AgreementSingle, AgreementSingle},
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: AgreementSingle,

		verdicts: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Worst(test.verdicts))
		})
	}
}
