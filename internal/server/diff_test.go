package server

import (
	"strings"
	"testing"
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
