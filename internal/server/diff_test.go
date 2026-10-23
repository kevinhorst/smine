package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffLines(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want []diffLine
	}{
		{
			name: "equal-inputs-all-same",
			old:  "a\nb",
			new:  "a\nb",
			want: []diffLine{{diffSame, "  a"}, {diffSame, "  b"}},
		},
		{
			name: "pure-insert",
			old:  "a",
			new:  "a\nb",
			want: []diffLine{{diffSame, "  a"}, {diffAdd, "+ b"}},
		},
		{
			name: "pure-delete",
			old:  "a\nb",
			new:  "a",
			want: []diffLine{{diffSame, "  a"}, {diffDel, "- b"}},
		},
		{
			name: "interleaved-change-keeps-lcs",
			old:  "a\nx\nc",
			new:  "a\ny\nc",
			want: []diffLine{{diffSame, "  a"}, {diffDel, "- x"}, {diffAdd, "+ y"}, {diffSame, "  c"}},
		},
		{
			name: "empty-old-vs-content",
			old:  "",
			new:  "a",
			want: []diffLine{{diffDel, "- "}, {diffAdd, "+ a"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := diffLines(test.old, test.new)
			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("line %d: got %v, want %v", i, got[i], test.want[i])
				}
			}
		})
	}
}

func TestDiffLinesFromPatch(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []diffLine

		patch string
	}

	tests := make([]*testCase, 0)

	// empty-patch-nil
	tests = append(tests, &testCase{
		_id:       "empty-patch-nil",
		_expected: nil,

		patch: "",
	})

	// file-headers
	tests = append(tests, &testCase{
		_id: "file-headers",
		_expected: []diffLine{
			{Kind: diffFile, Text: "diff --git a/x.go b/x.go"},
			{Kind: diffFile, Text: "--- a/x.go"},
			{Kind: diffFile, Text: "+++ b/x.go"},
		},

		patch: "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n",
	})

	// hunk-header
	tests = append(tests, &testCase{
		_id: "hunk-header",
		_expected: []diffLine{
			{Kind: diffHunk, Text: "@@ -1,2 +1,2 @@ func main()"},
		},

		patch: "@@ -1,2 +1,2 @@ func main()\n",
	})

	// add-and-del
	tests = append(tests, &testCase{
		_id: "add-and-del",
		_expected: []diffLine{
			{Kind: diffDel, Text: "-old"},
			{Kind: diffAdd, Text: "+new"},
		},

		patch: "-old\n+new\n",
	})

	// context-line-same
	tests = append(tests, &testCase{
		_id: "context-line-same",
		_expected: []diffLine{
			{Kind: diffSame, Text: " unchanged"},
			{Kind: diffSame, Text: ""},
		},

		patch: " unchanged\n\n",
	})

	// index-line-same
	tests = append(tests, &testCase{
		_id: "index-line-same",
		_expected: []diffLine{
			{Kind: diffSame, Text: "index abc1234..def5678 100644"},
			{Kind: diffSame, Text: "new file mode 100644"},
		},

		patch: "index abc1234..def5678 100644\nnew file mode 100644",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, diffLinesFromPatch(test.patch))
		})
	}
}

func TestCompactDiff(t *testing.T) {
	t.Run("no-changes-returns-nil", func(t *testing.T) {
		if got := compactDiff(diffLines("a\nb", "a\nb"), 2); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})

	t.Run("context-window-kept-around-change", func(t *testing.T) {
		got := compactDiff(diffLines("a\nb\nc\nd\ne", "a\nb\nX\nd\ne"), 1)
		want := []string{"  ⋯", "  b", "- c", "+ X", "  d", "  ⋯"}
		if formatDiff(got) != strings.Join(want, "\n") {
			t.Fatalf("got %q, want %q", formatDiff(got), strings.Join(want, "\n"))
		}
	})

	t.Run("long-same-run-collapses-to-one-skip", func(t *testing.T) {
		old := "X\na\nb\nc\nd\ne\nf\ng"
		got := compactDiff(diffLines(old, strings.Replace(old, "X", "Y", 1)), 1)
		skips := 0
		for _, line := range got {
			if line.Kind == diffSkip {
				skips++
			}
		}
		if skips != 1 {
			t.Fatalf("got %d skip lines in %v, want 1", skips, got)
		}
	})
}
