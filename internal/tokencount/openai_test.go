package tokencount

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiktoken-go/tokenizer"
)

func TestEncodingApprox(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		model string
	}

	tests := make([]*testCase, 0)

	// gpt-5-5
	tests = append(tests, &testCase{
		_id:       "gpt-5-5",
		_expected: false,

		model: "gpt-5.5",
	})

	// gpt-4o-mini
	tests = append(tests, &testCase{
		_id:       "gpt-4o-mini",
		_expected: false,

		model: "gpt-4o-mini",
	})

	// o3
	tests = append(tests, &testCase{
		_id:       "o3",
		_expected: false,

		model: "o3",
	})

	// gpt-6-astra
	tests = append(tests, &testCase{
		_id:       "gpt-6-astra",
		_expected: true,

		model: "gpt-6-astra",
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: true,

		model: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, encodingApprox(test.model))
		})
	}
}

func TestO200kCount(t *testing.T) {
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	require.NoError(t, err)

	ids, _, err := codec.Encode("hello world")
	require.NoError(t, err)

	count, err := codec.Count("hello world")
	require.NoError(t, err)

	emptyCount, err := codec.Count("")
	require.NoError(t, err)

	assert.Equal(t, 2, count)
	assert.Len(t, ids, count)
	assert.Equal(t, 0, emptyCount)
}
