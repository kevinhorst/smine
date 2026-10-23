package skilleval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	guardAssistantLine  = `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]},"parent_tool_use_id":null}` + "\n"
	guardOpusInitLine   = `{"type":"system","subtype":"init","model":"claude-opus-5-5","tools":["Read"],"mcp_servers":[]}` + "\n"
	guardSonnetInitLine = `{"type":"system","subtype":"init","model":"claude-sonnet-5[1m]","tools":["Read"],"mcp_servers":[]}` + "\n"
	guardTaskLine       = `{"type":"system","subtype":"task_started","message":"Workflow round-1 started"}` + "\n"
)

// newTestGuard returns a guard for a sonnet arm writing into a fresh capture
// file, and the context its cancel ends.
func newTestGuard(t *testing.T) (*initGuard, context.Context) {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), streamFile))
	require.NoError(t, err)
	t.Cleanup(func() { _ = out.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	guard := &initGuard{arm: &ManifestArm{Model: "sonnet"}, cancel: cancel, out: out}
	return guard, ctx
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// writeChunks writes each chunk through the guard, stopping at the first error.
func writeChunks(guard *initGuard, chunks []string) error {
	for _, chunk := range chunks {
		if _, err := guard.Write([]byte(chunk)); err != nil {
			return err
		}
	}
	return nil
}

func TestInitGuard(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedCancelled bool
		_expectedChecked   bool
		_expectedProblems  []string

		chunks []string
		ctx    context.Context
		guard  *initGuard
	}

	tests := make([]*testCase, 0)

	// matching-model-passes
	matchingGuard, matchingCtx := newTestGuard(t)
	tests = append(tests, &testCase{
		_id:                "matching-model-passes",
		_expectedCancelled: false,
		_expectedChecked:   true,
		_expectedProblems:  nil,

		chunks: []string{guardSonnetInitLine, guardAssistantLine},
		ctx:    matchingCtx,
		guard:  matchingGuard,
	})

	// other-model-cancels
	otherGuard, otherCtx := newTestGuard(t)
	tests = append(tests, &testCase{
		_id:                "other-model-cancels",
		_expectedCancelled: true,
		_expectedChecked:   true,
		_expectedProblems:  []string{"requested model sonnet but the cell ran claude-opus-5-5"},

		chunks: []string{guardOpusInitLine, guardAssistantLine},
		ctx:    otherCtx,
		guard:  otherGuard,
	})

	// init-split-across-writes
	splitGuard, splitCtx := newTestGuard(t)
	tests = append(tests, &testCase{
		_id:                "init-split-across-writes",
		_expectedCancelled: true,
		_expectedChecked:   true,
		_expectedProblems:  []string{"requested model sonnet but the cell ran claude-opus-5-5"},

		chunks: []string{guardTaskLine + guardOpusInitLine[:20], guardOpusInitLine[20:50], guardOpusInitLine[50:]},
		ctx:    splitCtx,
		guard:  splitGuard,
	})

	// non-init-lines-ignored
	ignoredGuard, ignoredCtx := newTestGuard(t)
	tests = append(tests, &testCase{
		_id:                "non-init-lines-ignored",
		_expectedCancelled: false,
		_expectedChecked:   false,
		_expectedProblems:  nil,

		chunks: []string{guardTaskLine, guardAssistantLine, "not json\n"},
		ctx:    ignoredCtx,
		guard:  ignoredGuard,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			require.NoError(t, writeChunks(test.guard, test.chunks))
			assert.Equal(t, test._expectedCancelled, test.ctx.Err() != nil)
			assert.Equal(t, test._expectedChecked, test.guard.isChecked)
			assert.Equal(t, test._expectedProblems, test.guard.problems)
			assert.Equal(t, strings.Join(test.chunks, ""), readTestFile(t, test.guard.out.Name()))
		})
	}
}
