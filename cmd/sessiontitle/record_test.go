package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveRecord(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedRecord  *SessionRecord
		_shouldPass      bool

		path string
	}

	dir := t.TempDir()
	full := &SessionRecord{Fleet: "T1-MA", Named: true, Turn: Turn{IsDelivered: true, State: "DESIGN-READY"}}

	tests := make([]*testCase, 0)

	// empty-record
	path := filepath.Join(dir, "a0000001.json")
	require.NoError(t, saveRecord(path, &SessionRecord{}))
	tests = append(tests, &testCase{
		_id:              "empty-record",
		_expectedContent: `{"named":false,"turn":{}}` + "\n",
		_expectedRecord:  &SessionRecord{},
		_shouldPass:      true,

		path: path,
	})

	// round-trip-equal
	path = filepath.Join(dir, "a0000002.json")
	require.NoError(t, saveRecord(path, full))
	tests = append(tests, &testCase{
		_id:              "round-trip-equal",
		_expectedContent: `{"fleet":"T1-MA","named":true,"turn":{"delivered":true,"state":"DESIGN-READY"}}` + "\n",
		_expectedRecord:  full,
		_shouldPass:      true,

		path: path,
	})

	// missing-file-nil
	tests = append(tests, &testCase{
		_id:              "missing-file-nil",
		_expectedContent: "",
		_expectedRecord:  nil,
		_shouldPass:      true,

		path: filepath.Join(dir, "a0000003.json"),
	})

	// damaged-file-fails
	path = filepath.Join(dir, "a0000004.json")
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o644))
	tests = append(tests, &testCase{
		_id:              "damaged-file-fails",
		_expectedContent: "{",
		_expectedRecord:  nil,
		_shouldPass:      false,

		path: path,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := loadRecord(test.path)
			content, _ := os.ReadFile(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedContent, string(content))
			assert.Equal(t, test._expectedRecord, record)
		})
	}
}
