package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripSkillFrontmatter(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		src string
	}

	tests := make([]*testCase, 0)

	// leading-frontmatter-removed
	tests = append(tests, &testCase{
		_id:       "leading-frontmatter-removed",
		_expected: "# Title\n",

		src: "---\nname: x\ndescription: y\n---\n# Title\n",
	})

	// no-frontmatter-unchanged
	tests = append(tests, &testCase{
		_id:       "no-frontmatter-unchanged",
		_expected: "# Title\n\ntext\n",

		src: "# Title\n\ntext\n",
	})

	// mid-document-rule-untouched
	tests = append(tests, &testCase{
		_id:       "mid-document-rule-untouched",
		_expected: "# Title\n\n---\ntext\n---\n",

		src: "# Title\n\n---\ntext\n---\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, string(stripSkillFrontmatter([]byte(test.src))))
		})
	}
}

func TestRenderSkillMarkdown(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    string
		_notExpected string

		src string
	}

	tests := make([]*testCase, 0)

	// entry-gate-classified
	tests = append(tests, &testCase{
		_id:          "entry-gate-classified",
		_expected:    `<div class="skill-entry skill-entry-gate" id="skill-foo-bar-001">`,
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[gate]` **Title** — body text.\n",
	})

	// entry-head-badges
	tests = append(tests, &testCase{
		_id:          "entry-head-badges",
		_expected:    `<div class="skill-entry-head"><span class="skill-entry-id">SKILL-FOO-BAR-001</span><span class="skill-entry-chip chip-gate">gate</span></div>`,
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[gate]` **Title** — body text.\n",
	})

	// entry-title-stripped
	tests = append(tests, &testCase{
		_id:          "entry-title-stripped",
		_expected:    `<p class="skill-entry-title"><strong>Title</strong> — body text.</p>`,
		_notExpected: "[gate]",

		src: "**SKILL-FOO-BAR-001** `[gate]` **Title** — body text.\n",
	})

	// entry-payload-classified
	tests = append(tests, &testCase{
		_id:          "entry-payload-classified",
		_expected:    `<div class="skill-entry skill-entry-payload" id="skill-foo-bar-002a">`,
		_notExpected: "",

		src: "**SKILL-FOO-BAR-002a** `[payload]` **Template** — the template.\n",
	})

	// entry-without-tag-plain
	tests = append(tests, &testCase{
		_id:          "entry-without-tag-plain",
		_expected:    `<div class="skill-entry skill-entry-plain" id="skill-foo-bar-003">`,
		_notExpected: "skill-entry-chip",

		src: "**SKILL-FOO-BAR-003** **Title** — body without a tag.\n",
	})

	// ordinary-paragraph-stock
	tests = append(tests, &testCase{
		_id:          "ordinary-paragraph-stock",
		_expected:    "<p>plain <strong>bold</strong> text.</p>",
		_notExpected: "skill-entry",

		src: "plain **bold** text.\n",
	})

	// malformed-id-degrades
	tests = append(tests, &testCase{
		_id:          "malformed-id-degrades",
		_expected:    "<p><strong>SKILL-foo-001</strong>",
		_notExpected: "skill-entry",

		src: "**SKILL-foo-001** `[gate]` **Title** — lower-case id is no entry.\n",
	})

	// fenced-block-untouched
	tests = append(tests, &testCase{
		_id:          "fenced-block-untouched",
		_expected:    "<pre><code>**SKILL-FOO-BAR-001** not an entry\n</code></pre>",
		_notExpected: "skill-entry",

		src: "```\n**SKILL-FOO-BAR-001** not an entry\n```\n",
	})

	// perm-rule-code-span-fallback
	tests = append(tests, &testCase{
		_id:          "perm-rule-code-span-fallback",
		_expected:    "<code>Bash(jq *)</code>",
		_notExpected: "",

		src: "allowed: `Bash(jq *)` inline.\n",
	})

	// pipe-table-renders-table
	tests = append(tests, &testCase{
		_id:          "pipe-table-renders-table",
		_expected:    "<table>",
		_notExpected: "| :--- |",

		src: "| Direction | Goal |\n| :--- | :--- |\n| a | b |\n",
	})

	// Run tests
	server := newTestServer(t, &Options{})
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := server.renderSkillMarkdown([]byte(test.src))
			require.NoError(t, err)
			assert.Contains(t, string(rendered.Body), test._expected)
			if test._notExpected != "" {
				assert.NotContains(t, string(rendered.Body), test._notExpected)
			}
		})
	}
}

func TestRenderSkillMarkdownEntryBlocks(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    string
		_notExpected string

		src string
	}

	tests := make([]*testCase, 0)

	// bullets-absorbed-into-box
	tests = append(tests, &testCase{
		_id:          "bullets-absorbed-into-box",
		_expected:    "<ul>\n<li>Why: because.</li>\n</ul>\n</div>",
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[gate]` **Title** — body.\n* Why: because.\n",
	})

	// table-absorbed-into-box
	tests = append(tests, &testCase{
		_id:          "table-absorbed-into-box",
		_expected:    "</table>\n</div>",
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[prose]` **Table** — the table:\n\n| A | B |\n| :--- | :--- |\n| 1 | 2 |\n",
	})

	// fence-absorbed-into-box
	tests = append(tests, &testCase{
		_id:          "fence-absorbed-into-box",
		_expected:    "</code></pre>\n</div>",
		_notExpected: "",

		src: "**SKILL-FOO-BAR-002** `[payload]` **Payload** — template:\n\n```json\n{}\n```\n",
	})

	// next-entry-bounds-block
	tests = append(tests, &testCase{
		_id:          "next-entry-bounds-block",
		_expected:    `</div>` + "\n" + `<div class="skill-entry skill-entry-step" id="skill-foo-bar-002">`,
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[gate]` **First** — one.\n\n**SKILL-FOO-BAR-002** `[step]` **Second** — two.\n",
	})

	// heading-bounds-block
	tests = append(tests, &testCase{
		_id:          "heading-bounds-block",
		_expected:    "</div>\n<h2",
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[gate]` **First** — one.\n\n## Next section\n\ntext after.\n",
	})

	// trailing-paragraph-absorbed
	tests = append(tests, &testCase{
		_id:          "trailing-paragraph-absorbed",
		_expected:    "<p>loose continuation.</p>\n</div>",
		_notExpected: "",

		src: "**SKILL-FOO-BAR-001** `[gate]` **First** — one.\n\nloose continuation.\n",
	})

	// Run tests
	server := newTestServer(t, &Options{})
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := server.renderSkillMarkdown([]byte(test.src))
			require.NoError(t, err)
			assert.Contains(t, string(rendered.Body), test._expected)
			if test._notExpected != "" {
				assert.NotContains(t, string(rendered.Body), test._notExpected)
			}
		})
	}
}

func TestRenderSkillMarkdownLinkify(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    string
		_notExpected string

		src string
	}

	entry := "**SKILL-FOO-MODES-002** `[gate]` **Anchor** — target entry.\n\n"

	tests := make([]*testCase, 0)

	// on-page-short-ref-linked
	tests = append(tests, &testCase{
		_id:          "on-page-short-ref-linked",
		_expected:    `see <a href="#skill-foo-modes-002">MODES-002</a> for`,
		_notExpected: "",

		src: entry + "see MODES-002 for details.\n",
	})

	// range-ref-links-start
	tests = append(tests, &testCase{
		_id:          "range-ref-links-start",
		_expected:    `<a href="#skill-foo-modes-002">MODES-002..004</a>`,
		_notExpected: "",

		src: entry + "see MODES-002..004 for details.\n",
	})

	// off-page-token-untouched
	tests = append(tests, &testCase{
		_id:          "off-page-token-untouched",
		_expected:    "<p>see PROC-009 for details.</p>",
		_notExpected: `href="#skill-foo-proc-009"`,

		src: entry + "see PROC-009 for details.\n",
	})

	// token-in-code-span-untouched
	tests = append(tests, &testCase{
		_id:          "token-in-code-span-untouched",
		_expected:    "<code>MODES-002</code>",
		_notExpected: `<code><a`,

		src: entry + "see `MODES-002` for details.\n",
	})

	// longer-id-tail-not-linked
	tests = append(tests, &testCase{
		_id:          "longer-id-tail-not-linked",
		_expected:    "<p>see RULE-MODES-002 for details.</p>",
		_notExpected: "<a href",

		src: entry + "see RULE-MODES-002 for details.\n",
	})

	// context-link-rewritten
	tests = append(tests, &testCase{
		_id:          "context-link-rewritten",
		_expected:    `<a href="/context/rules/plan.md">the plan-dir rule</a>`,
		_notExpected: "AGENT_CONTEXT_DIR_DEFAULT",

		src: "see [the plan-dir rule]($AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md).\n",
	})

	// substituted-context-link-rewritten
	tests = append(tests, &testCase{
		_id:          "substituted-context-link-rewritten",
		_expected:    `<a href="/context/actions/implementing.md">the gates</a>`,
		_notExpected: "",

		src: "see [the gates](docs/actions/implementing.md).\n",
	})

	// external-link-untouched
	tests = append(tests, &testCase{
		_id:          "external-link-untouched",
		_expected:    `<a href="https://example.com/rules/plan.md">doc</a>`,
		_notExpected: "/context/",

		src: "see [doc](https://example.com/rules/plan.md).\n",
	})

	// Run tests
	server := newTestServer(t, &Options{})
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := server.renderSkillMarkdown([]byte(test.src))
			require.NoError(t, err)
			assert.Contains(t, string(rendered.Body), test._expected)
			if test._notExpected != "" {
				assert.NotContains(t, string(rendered.Body), test._notExpected)
			}
		})
	}
}

func TestRenderSkillMarkdownToc(t *testing.T) {
	server := newTestServer(t, &Options{})
	src := "# Skill\n\nintro.\n\n## When to use\n\ntext.\n\n## Args\n\ntext.\n\n### Sub\n\ntext.\n"

	rendered, err := server.renderSkillMarkdown([]byte(src))
	require.NoError(t, err)

	require.Len(t, rendered.Toc, 2)
	assert.Equal(t, "When to use", rendered.Toc[0].Title)
	assert.Equal(t, "Args", rendered.Toc[1].Title)
	assert.Contains(t, string(rendered.Body), `id="`+rendered.Toc[0].Anchor+`"`)
	assert.Contains(t, string(rendered.Body), `id="`+rendered.Toc[1].Anchor+`"`)
}

func TestRenderSkillMarkdownMermaid(t *testing.T) {
	server := newTestServer(t, &Options{})
	src := "```mermaid\nflowchart TD\n  A --> B\n```\n"

	rendered, err := server.renderSkillMarkdown([]byte(src))
	require.NoError(t, err)
	assert.Contains(t, string(rendered.Body), `<pre class="mermaid">flowchart TD`)
	assert.NotContains(t, string(rendered.Body), "language-mermaid")
}
