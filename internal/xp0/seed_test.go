package xp0

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescribe(t *testing.T) {
	type testCase struct {
		_id string

		want string
	}

	tests := make([]*testCase, 0)

	// pass-fixed-description
	test := &testCase{
		_id:  "pass-fixed-description",
		want: "xp0 experiment seed",
	}
	tests = append(tests, test)

	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test.want, Describe())
		})
	}
}
