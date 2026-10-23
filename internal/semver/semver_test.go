package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompare(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   int
		_shouldPass bool

		a string
		b string
	}

	tests := make([]*testCase, 0)

	// pass-equal-three-part
	tests = append(tests, &testCase{_id: "pass-equal-three-part", _expected: 0, _shouldPass: true, a: "1.2.3", b: "1.2.3"})

	// pass-lower-minor
	tests = append(tests, &testCase{_id: "pass-lower-minor", _expected: -1, _shouldPass: true, a: "1.0.9", b: "1.1.0"})

	// pass-higher-patch
	tests = append(tests, &testCase{_id: "pass-higher-patch", _expected: 1, _shouldPass: true, a: "1.0.2", b: "1.0.1"})

	// pass-two-part-below-three-part
	tests = append(tests, &testCase{_id: "pass-two-part-below-three-part", _expected: -1, _shouldPass: true, a: "1.7", b: "1.7.1"})

	// pass-two-part-equal-to-padded
	tests = append(tests, &testCase{_id: "pass-two-part-equal-to-padded", _expected: 0, _shouldPass: true, a: "1.7", b: "1.7.0"})

	// fail-unparseable-component
	tests = append(tests, &testCase{_id: "fail-unparseable-component", _expected: 0, _shouldPass: false, a: "1.x", b: "1.0"})

	// fail-one-component
	tests = append(tests, &testCase{_id: "fail-one-component", _expected: 0, _shouldPass: false, a: "1.0", b: "3"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			order, err := Compare(test.a, test.b)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, order)
		})
	}
}

func TestParse(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []int
		_shouldPass bool

		version string
	}

	tests := make([]*testCase, 0)

	// pass-three-part
	tests = append(tests, &testCase{_id: "pass-three-part", _expected: []int{1, 0, 1}, _shouldPass: true, version: "1.0.1"})

	// pass-two-part
	tests = append(tests, &testCase{_id: "pass-two-part", _expected: []int{1, 7}, _shouldPass: true, version: "1.7"})

	// fail-one-part
	tests = append(tests, &testCase{_id: "fail-one-part", _expected: nil, _shouldPass: false, version: "1"})

	// fail-four-parts
	tests = append(tests, &testCase{_id: "fail-four-parts", _expected: nil, _shouldPass: false, version: "1.0.0.1"})

	// fail-negative-component
	tests = append(tests, &testCase{_id: "fail-negative-component", _expected: nil, _shouldPass: false, version: "1.-1.0"})

	// fail-letters
	tests = append(tests, &testCase{_id: "fail-letters", _expected: nil, _shouldPass: false, version: "1.a.0"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			components, err := Parse(test.version)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, components)
		})
	}
}
