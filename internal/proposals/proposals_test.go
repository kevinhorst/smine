package proposals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validFile = `{
  "kind": "routines",
  "source": "sessions/proposals/routines.json",
  "updated": "2026-07-11",
  "note": "Execution-constraint key.",
  "groups": [
    {
      "title": "Proposals",
      "proposals": [
        {
          "id": "nightly-session-analysis",
          "title": "nightly-session-analysis",
          "status": "proposed",
          "tags": ["local"],
          "target": "smine",
          "change": "Wrap smine in a nightly schedule.",
          "fields": [
            {"label": "Purpose", "text": "Run the retrospective pipeline."},
            {"label": "Cadence", "text": "Nightly."}
          ],
          "evidence": [
            {
              "title": "Manual nightly trigger",
              "dimension": "routine-candidate",
              "sessions": [
                {"id": "ea25febf-3bcc-4c51-a237-5c08cf602e49", "note": "batch 02 memory candidate"}
              ],
              "snippets": [
                {"kind": "violation", "lang": "go", "code": "func broken() {}", "source": "transcript"}
              ]
            }
          ]
        }
      ]
    }
  ]
}`

func writeProposalsFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestLoad(t *testing.T) {
	t.Run("valid-file-parsed", func(t *testing.T) {
		dir := t.TempDir()
		writeProposalsFile(t, dir, "routines.json", validFile)

		files, loadErrors, err := Load(dir)
		require.NoError(t, err)
		assert.Empty(t, loadErrors)
		require.Len(t, files, 1)
		assert.Equal(t, "routines", files[0].Kind)
		assert.Equal(t, "sessions/proposals/routines.json", files[0].Source)
		require.Len(t, files[0].Groups, 1)
		group := files[0].Groups[0]
		assert.Equal(t, "Proposals", group.Title)
		require.Len(t, group.Proposals, 1)
		proposal := group.Proposals[0]
		assert.Equal(t, "nightly-session-analysis", proposal.Id)
		assert.Equal(t, "proposed", proposal.Status)
		assert.Equal(t, []string{"local"}, proposal.Tags)
		assert.Equal(t, "smine", proposal.Target)
		assert.Equal(t, "Wrap smine in a nightly schedule.", proposal.Change)
		require.Len(t, proposal.Fields, 2)
		assert.Equal(t, "Purpose", proposal.Fields[0].Label)
		require.Len(t, proposal.Evidence, 1)
		evidence := proposal.Evidence[0]
		assert.Equal(t, "Manual nightly trigger", evidence.Title)
		assert.Equal(t, "routine-candidate", evidence.Dimension)
		assert.Equal(t, []Session{{Id: "ea25febf-3bcc-4c51-a237-5c08cf602e49", Note: "batch 02 memory candidate"}}, evidence.Sessions)
		require.Len(t, evidence.Snippets, 1)
		snippet := evidence.Snippets[0]
		assert.Equal(t, "violation", snippet.Kind)
		assert.Equal(t, "go", snippet.Lang)
		assert.Equal(t, "func broken() {}", snippet.Code)
		assert.Equal(t, "transcript", snippet.Source)
	})

	t.Run("legacy-string-evidence-load-error", func(t *testing.T) {
		dir := t.TempDir()
		writeProposalsFile(t, dir, "legacy.json", `{"kind": "style", "groups": [{"title": "G", "proposals": [{"title": "p", "evidence": ["a string"]}]}]}`)

		files, loadErrors, err := Load(dir)
		require.NoError(t, err)
		require.Len(t, loadErrors, 1)
		assert.Contains(t, loadErrors[0], "legacy.json")
		assert.Empty(t, files)
	})

	t.Run("malformed-file-recorded-others-load", func(t *testing.T) {
		dir := t.TempDir()
		writeProposalsFile(t, dir, "routines.json", validFile)
		writeProposalsFile(t, dir, "broken.json", "{not json")

		files, loadErrors, err := Load(dir)
		require.NoError(t, err)
		require.Len(t, loadErrors, 1)
		assert.Contains(t, loadErrors[0], "broken.json")
		require.Len(t, files, 1)
		assert.Equal(t, "routines", files[0].Kind)
	})

	t.Run("sorted-by-kind", func(t *testing.T) {
		dir := t.TempDir()
		writeProposalsFile(t, dir, "z.json", `{"kind": "style", "groups": []}`)
		writeProposalsFile(t, dir, "a.json", `{"kind": "workflows", "groups": []}`)

		files, loadErrors, err := Load(dir)
		require.NoError(t, err)
		assert.Empty(t, loadErrors)
		require.Len(t, files, 2)
		assert.Equal(t, "style", files[0].Kind)
		assert.Equal(t, "workflows", files[1].Kind)
	})

	t.Run("auto-apply-held-parsed", func(t *testing.T) {
		dir := t.TempDir()
		writeProposalsFile(t, dir, "style.json", `{"kind": "style", "groups": [{"title": "G", "proposals": [{"title": "held", "status": "proposed", "change": "c", "autoApplyHeld": {"date": "2026-07-30", "reason": "new routine"}}, {"title": "free", "status": "proposed", "change": "c"}]}]}`)

		files, loadErrors, err := Load(dir)
		require.NoError(t, err)
		assert.Empty(t, loadErrors)
		require.Len(t, files, 1)
		heldProposals := files[0].Groups[0].Proposals
		require.Len(t, heldProposals, 2)
		assert.Equal(t, AutoApplyHold{Date: "2026-07-30", Reason: "new routine"}, heldProposals[0].AutoApplyHeld)
		assert.Equal(t, AutoApplyHold{}, heldProposals[1].AutoApplyHeld)
	})

	t.Run("missing-dir-empty", func(t *testing.T) {
		files, loadErrors, err := Load(filepath.Join(t.TempDir(), "absent"))
		require.NoError(t, err)
		assert.Empty(t, loadErrors)
		assert.Empty(t, files)
	})

	t.Run("schema-json-skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeProposalsFile(t, dir, "routines.json", validFile)
		writeProposalsFile(t, dir, "schema.json", `{"not": "a proposal file"}`)

		files, loadErrors, err := Load(dir)
		require.NoError(t, err)
		assert.Empty(t, loadErrors)
		require.Len(t, files, 1)
		assert.Equal(t, "routines", files[0].Kind)
	})
}
