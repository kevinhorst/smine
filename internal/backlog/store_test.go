package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture is a canonical store (skills/concept/backlog/schema.json) with a
// second section, a dep and a rejected row.
const fixture = `{
  "archive": {
    "done": [
      {
        "id": "B-023",
        "date": "2026-09-18",
        "description": "Workflow invocation\nDone when: already done before backlog creation.",
        "link": "",
        "reason": "already done before backlog creation",
        "title": "Workflow invocation"
      }
    ],
    "rejected": [
      {
        "id": "B-099",
        "date": "2026-09-19",
        "description": "Bad idea\nDone when: never.",
        "link": "",
        "reason": "no",
        "title": "Bad idea"
      }
    ]
  },
  "changelog": [
    {
      "date": "2026-09-18",
      "delta": "Initial"
    }
  ],
  "sections": [
    {
      "items": [
        {
          "id": "B-001",
          "added": "2026-09-18",
          "deps": [],
          "description": "Contract sweeping defective -> misses <consumers>\nDone when: sweeps cover jq | sh.",
          "evidence": [
            "commit 935993b1"
          ],
          "link": "",
          "pri": "critical",
          "route": "/idea critical",
          "size": "L",
          "status": "shaped",
          "theme": "quality",
          "title": "Contract sweeping defective"
        },
        {
          "id": "B-002",
          "added": "2026-09-18",
          "deps": [
            "B-001"
          ],
          "description": "Audit-agent\nDone when: all errors land in a file.",
          "evidence": [],
          "link": "",
          "pri": "critical",
          "route": "/fdesign",
          "size": "M",
          "status": "shaped",
          "theme": "quality",
          "title": "Audit-agent"
        }
      ],
      "name": "Defense"
    },
    {
      "items": [
        {
          "id": "B-005",
          "added": "2026-09-18",
          "deps": [
            "B-001"
          ],
          "description": "Railroad-review lanes refactor\nDone when: lanes are refactored.",
          "evidence": [],
          "link": "plans/x",
          "pri": "critical",
          "route": "/fdesign change",
          "size": "L",
          "status": "review",
          "theme": "rr",
          "title": "Railroad-review lanes refactor"
        }
      ],
      "name": "Railroad"
    }
  ],
  "updated": "2026-09-18"
}
`

func writeStore(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backlog.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// withChangelogRow appends one changelog object after the fixture's Initial
// row and restamps updated — the shape every board write must produce.
func withChangelogRow(content, delta, today string) string {
	inserted := "\"delta\": \"Initial\"\n    },\n    {\n      \"date\": \"" + today + "\",\n      \"delta\": \"" + delta + "\"\n    }"
	content = strings.Replace(content, "\"delta\": \"Initial\"\n    }", inserted, 1)
	return strings.Replace(content, "\"updated\": \"2026-09-18\"", "\"updated\": \""+today+"\"", 1)
}

// withEmptySection appends an empty section after the fixture's last one, in canonical encoding.
func withEmptySection(content, name string) string {
	appended := "\n    },\n    {\n      \"items\": [],\n      \"name\": \"" + name + "\"\n    }\n  ],\n  \"updated\""
	return strings.Replace(content, "\n    }\n  ],\n  \"updated\"", appended, 1)
}

// writeStaleStore writes the store with an epoch mtime, so a test can tell whether a write touched it.
func writeStaleStore(t *testing.T, content string) string {
	t.Helper()
	path := writeStore(t, content)
	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(path, epoch, epoch))
	return path
}

// sectionSummary renders the store as "<section>: <ids> | …" in store order; empty when it does not load.
func sectionSummary(path string) string {
	store, err := Load(path)
	if err != nil {
		return ""
	}

	parts := make([]string, 0, len(store.Sections))
	for _, section := range store.Sections {
		ids := make([]string, 0, len(section.Items))
		for _, item := range section.Items {
			ids = append(ids, item.Id)
		}
		parts = append(parts, section.Name+": "+strings.Join(ids, ","))
	}
	return strings.Join(parts, " | ")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestLoad(t *testing.T) {
	store, err := Load(writeStore(t, fixture))
	require.NoError(t, err)

	assert.Equal(t, "2026-09-18", store.Updated)
	assert.Equal(t, 3, store.Active())
	assert.Equal(t, "B-100", store.NextId())
	require.Len(t, store.Sections, 2)
	assert.Equal(t, "Defense", store.Sections[0].Name)

	item, found := store.Find("B-002")
	require.True(t, found)
	assert.Equal(t, "Defense", item.Section)
	assert.Equal(t, []string{"B-001"}, item.Deps)
	assert.Equal(t, "Audit-agent\nDone when: all errors land in a file.", item.Description)
	assert.Equal(t, "/fdesign", item.Route)
	assert.Equal(t, "shaped", item.Status)

	first, _ := store.Find("B-001")
	assert.Equal(t, []string{"commit 935993b1"}, first.Evidence)
	assert.Empty(t, first.Deps)
	_, found = store.Find("B-999")
	assert.False(t, found)
	assert.Len(t, store.Items(), 3)
	require.Len(t, store.Archive.Done, 1)
	assert.Equal(t, "already done before backlog creation", store.Archive.Done[0].Reason)
	require.Len(t, store.Archive.Rejected, 1)
	assert.Equal(t, "no", store.Archive.Rejected[0].Reason)
}

func TestLoad_missing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoad_invalid(t *testing.T) {
	type testCase struct {
		_id string

		content string
	}
	tests := make([]*testCase, 0)

	// unknown-top-level-key
	tests = append(tests, &testCase{_id: "unknown-top-level-key", content: strings.Replace(fixture, "\"updated\"", "\"active\": 3,\n  \"updated\"", 1)})

	// blank-description
	tests = append(tests, &testCase{_id: "blank-description", content: strings.Replace(fixture, `"description": "Audit-agent\nDone when: all errors land in a file."`, `"description": "  "`, 1)})

	// duplicate-id-across-archive
	tests = append(tests, &testCase{_id: "duplicate-id-across-archive", content: strings.Replace(fixture, `"id": "B-023"`, `"id": "B-001"`, 1)})

	// unknown-dep
	tests = append(tests, &testCase{_id: "unknown-dep", content: strings.Replace(fixture, "\"B-001\"\n", "\"B-777\"\n", 1)})

	// bad-pri
	tests = append(tests, &testCase{_id: "bad-pri", content: strings.Replace(fixture, `"pri": "critical"`, `"pri": "P1"`, 1)})

	// bad-status
	tests = append(tests, &testCase{_id: "bad-status", content: strings.Replace(fixture, `"status": "review"`, `"status": "in-pipeline"`, 1)})

	// malformed-json
	tests = append(tests, &testCase{_id: "malformed-json", content: `{"sections": [`})

	// blank-section-name
	tests = append(tests, &testCase{_id: "blank-section-name", content: strings.Replace(fixture, `"name": "Defense"`, `"name": " "`, 1)})

	// duplicate-section-name
	tests = append(tests, &testCase{_id: "duplicate-section-name", content: strings.Replace(fixture, `"name": "Railroad"`, `"name": "Defense"`, 1)})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			require.NotEqual(t, fixture, test.content, "the case must alter the fixture")
			_, err := Load(writeStore(t, test.content))
			assert.ErrorIs(t, err, ErrFormat)
		})
	}
}

func TestSetStatus(t *testing.T) {
	t.Run("rewrites-status-and-appends-changelog", func(t *testing.T) {
		path := writeStore(t, fixture)
		require.NoError(t, SetStatus("B-002", path, StatusPlanning))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		today := time.Now().Format(dateLayout)
		expected := withChangelogRow(fixture, "B-002 status shaped → planning (board)", today)
		expected = strings.Replace(expected, "\"status\": \"shaped\",\n          \"theme\": \"quality\",\n          \"title\": \"Audit-agent\"", "\"status\": \"planning\",\n          \"theme\": \"quality\",\n          \"title\": \"Audit-agent\"", 1)
		// byte-exact on purpose: the canonical form is the contract, not JSON equivalence
		assert.Equal(t, strings.Split(expected, "\n"), strings.Split(string(after), "\n"))

		store, err := Load(path)
		require.NoError(t, err)
		item, _ := store.Find("B-002")
		assert.Equal(t, StatusPlanning, item.Status)
		assert.NotEqual(t, "2026-09-18", store.Updated)
	})

	t.Run("invalid-status", func(t *testing.T) {
		err := SetStatus("B-002", writeStore(t, fixture), "in-pipeline")
		assert.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("done-refused", func(t *testing.T) {
		err := SetStatus("B-002", writeStore(t, fixture), StatusDone)
		assert.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("rejected-refused", func(t *testing.T) {
		err := SetStatus("B-002", writeStore(t, fixture), StatusRejected)
		assert.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("parked-written", func(t *testing.T) {
		path := writeStore(t, fixture)
		require.NoError(t, SetStatus("B-002", path, StatusParked))

		store, err := Load(path)
		require.NoError(t, err)
		item, _ := store.Find("B-002")
		assert.Equal(t, StatusParked, item.Status)
		assert.Equal(t, "B-002 status shaped → parked (board)", store.Changelog[len(store.Changelog)-1].Delta)
	})

	t.Run("unknown-id", func(t *testing.T) {
		err := SetStatus("B-999", writeStore(t, fixture), StatusShaped)
		assert.ErrorIs(t, err, ErrItemNotFound)
	})
}

func TestSetPri(t *testing.T) {
	t.Run("rewrites-pri-and-appends-changelog", func(t *testing.T) {
		path := writeStore(t, fixture)
		require.NoError(t, SetPri("B-002", path, "high"))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		today := time.Now().Format(dateLayout)
		expected := withChangelogRow(fixture, "B-002 pri critical → high (board)", today)
		expected = strings.Replace(expected, "\"pri\": \"critical\",\n          \"route\": \"/fdesign\",", "\"pri\": \"high\",\n          \"route\": \"/fdesign\",", 1)
		// byte-exact on purpose: the canonical form is the contract, not JSON equivalence
		assert.Equal(t, strings.Split(expected, "\n"), strings.Split(string(after), "\n"))
	})

	t.Run("invalid-pri", func(t *testing.T) {
		path := writeStore(t, fixture)
		err := SetPri("B-002", path, "P1")
		assert.ErrorIs(t, err, ErrInvalidPri)
		assert.Equal(t, strings.Split(fixture, "\n"), strings.Split(readFile(t, path), "\n"))
	})

	t.Run("unknown-id", func(t *testing.T) {
		path := writeStore(t, fixture)
		err := SetPri("B-999", path, "high")
		assert.ErrorIs(t, err, ErrItemNotFound)
		assert.Equal(t, strings.Split(fixture, "\n"), strings.Split(readFile(t, path), "\n"))
	})
}

func TestSetRouteSkill(t *testing.T) {
	t.Run("rewrites-route-and-logs-the-old-one", func(t *testing.T) {
		path := writeStore(t, fixture)
		require.NoError(t, SetRouteSkill("B-002", path, "idea"))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		today := time.Now().Format(dateLayout)
		expected := withChangelogRow(fixture, "B-002 route /fdesign → /idea (board)", today)
		expected = strings.Replace(expected, "\"pri\": \"critical\",\n          \"route\": \"/fdesign\",", "\"pri\": \"critical\",\n          \"route\": \"/idea\",", 1)
		// byte-exact on purpose: the canonical form is the contract, not JSON equivalence
		assert.Equal(t, strings.Split(expected, "\n"), strings.Split(string(after), "\n"))
	})

	t.Run("blank-skill-refused", func(t *testing.T) {
		path := writeStore(t, fixture)
		err := SetRouteSkill("B-002", path, "")
		assert.ErrorIs(t, err, ErrInvalidRoute)
		assert.Equal(t, strings.Split(fixture, "\n"), strings.Split(readFile(t, path), "\n"))
	})

	t.Run("slash-skill-refused", func(t *testing.T) {
		path := writeStore(t, fixture)
		err := SetRouteSkill("B-002", path, "/idea critical")
		assert.ErrorIs(t, err, ErrInvalidRoute)
		assert.Equal(t, strings.Split(fixture, "\n"), strings.Split(readFile(t, path), "\n"))
	})

	t.Run("unknown-id", func(t *testing.T) {
		path := writeStore(t, fixture)
		err := SetRouteSkill("B-999", path, "idea")
		assert.ErrorIs(t, err, ErrItemNotFound)
		assert.Equal(t, strings.Split(fixture, "\n"), strings.Split(readFile(t, path), "\n"))
	})
}

func TestRecordRun(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDelta  string
		_expectedErr    error
		_expectedLink   string
		_expectedStatus string

		id       string
		lookupId string
		status   string
	}

	tests := make([]*testCase, 0)

	// status-and-link
	tests = append(tests, &testCase{
		_id:             "status-and-link",
		_expectedDelta:  "B-001 link claude-routines/backlog-idea-2026-09-27, status shaped → planning (routine)",
		_expectedErr:    nil,
		_expectedLink:   "claude-routines/backlog-idea-2026-09-27",
		_expectedStatus: StatusPlanning,

		id:       "B-001",
		lookupId: "B-001",
		status:   StatusPlanning,
	})

	// same-status-link-only
	tests = append(tests, &testCase{
		_id:             "same-status-link-only",
		_expectedDelta:  "B-005 link claude-routines/backlog-idea-2026-09-27 (routine)",
		_expectedErr:    nil,
		_expectedLink:   "claude-routines/backlog-idea-2026-09-27",
		_expectedStatus: StatusReview,

		id:       "B-005",
		lookupId: "B-005",
		status:   StatusReview,
	})

	// no-stage-link-only
	tests = append(tests, &testCase{
		_id:             "no-stage-link-only",
		_expectedDelta:  "B-001 link claude-routines/backlog-idea-2026-09-27 (routine)",
		_expectedErr:    nil,
		_expectedLink:   "claude-routines/backlog-idea-2026-09-27",
		_expectedStatus: StatusShaped,

		id:       "B-001",
		lookupId: "B-001",
		status:   "",
	})

	// invalid-status
	tests = append(tests, &testCase{
		_id:             "invalid-status",
		_expectedDelta:  "Initial",
		_expectedErr:    ErrInvalidStatus,
		_expectedLink:   "",
		_expectedStatus: StatusShaped,

		id:       "B-001",
		lookupId: "B-001",
		status:   StatusDone,
	})

	// unknown-item
	tests = append(tests, &testCase{
		_id:             "unknown-item",
		_expectedDelta:  "Initial",
		_expectedErr:    ErrItemNotFound,
		_expectedLink:   "",
		_expectedStatus: StatusShaped,

		id:       "B-999",
		lookupId: "B-001",
		status:   StatusPlanning,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := writeStore(t, fixture)

			err := RecordRun(test.id, "claude-routines/backlog-idea-2026-09-27", path, test.status)

			require.ErrorIs(t, err, test._expectedErr)
			store, loadErr := Load(path)
			require.NoError(t, loadErr)
			item, _ := store.Find(test.lookupId)
			assert.Equal(t, test._expectedLink, item.Link)
			assert.Equal(t, test._expectedStatus, item.Status)
			assert.Equal(t, test._expectedDelta, store.Changelog[len(store.Changelog)-1].Delta)
		})
	}
}

func TestItemIdOf(t *testing.T) {
	assert.Equal(t, "B-001", ItemIdOf("critical B-001: Contract sweeping\n\nbody"))
	assert.Equal(t, "B-0051", ItemIdOf("B-0051: long id"))
	assert.Empty(t, ItemIdOf("focus on tests"))
	assert.Empty(t, ItemIdOf("SUB-005 is not an item"))
}

func TestStageForSkill(t *testing.T) {
	assert.Equal(t, StatusPlanning, StageForSkill("idea"))
	assert.Equal(t, StatusImplementing, StageForSkill("fimplement"))
	assert.Empty(t, StageForSkill("spec-drift"))
}

func TestMoveToArchive(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedActive   int
		_expectedDelta    string
		_expectedDone     []ArchiveRow
		_expectedErr      error
		_expectedFound    bool
		_expectedRejected []ArchiveRow

		id         string
		resolution string
		status     string
	}

	today := time.Now().Format(dateLayout)
	fixtureDone := ArchiveRow{
		Id:          "B-023",
		Date:        "2026-09-18",
		Description: "Workflow invocation\nDone when: already done before backlog creation.",
		Link:        "",
		Reason:      "already done before backlog creation",
		Title:       "Workflow invocation",
	}
	fixtureRejected := ArchiveRow{
		Id:          "B-099",
		Date:        "2026-09-19",
		Description: "Bad idea\nDone when: never.",
		Link:        "",
		Reason:      "no",
		Title:       "Bad idea",
	}
	movedDone := ArchiveRow{
		Id:          "B-005",
		Date:        today,
		Description: "Railroad-review lanes refactor\nDone when: lanes are refactored.",
		Link:        "plans/x",
		Reason:      "done (board)",
		Title:       "Railroad-review lanes refactor",
	}
	movedRejected := movedDone
	movedRejected.Reason = "rejected (board)"

	tests := make([]*testCase, 0)

	// done
	tests = append(tests, &testCase{
		_id:               "done",
		_expectedActive:   2,
		_expectedDelta:    "B-005 done → archive (board): done (board)",
		_expectedDone:     []ArchiveRow{fixtureDone, movedDone},
		_expectedErr:      nil,
		_expectedFound:    false,
		_expectedRejected: []ArchiveRow{fixtureRejected},

		id:         "B-005",
		resolution: "done (board)",
		status:     StatusDone,
	})

	// rejected
	tests = append(tests, &testCase{
		_id:               "rejected",
		_expectedActive:   2,
		_expectedDelta:    "B-005 rejected → archive (board): rejected (board)",
		_expectedDone:     []ArchiveRow{fixtureDone},
		_expectedErr:      nil,
		_expectedFound:    false,
		_expectedRejected: []ArchiveRow{fixtureRejected, movedRejected},

		id:         "B-005",
		resolution: "rejected (board)",
		status:     StatusRejected,
	})

	// parked
	tests = append(tests, &testCase{
		_id:               "parked",
		_expectedActive:   3,
		_expectedDelta:    "Initial",
		_expectedDone:     []ArchiveRow{fixtureDone},
		_expectedErr:      ErrInvalidStatus,
		_expectedFound:    true,
		_expectedRejected: []ArchiveRow{fixtureRejected},

		id:         "B-005",
		resolution: "x",
		status:     StatusParked,
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:               "unknown-id",
		_expectedActive:   3,
		_expectedDelta:    "Initial",
		_expectedDone:     []ArchiveRow{fixtureDone},
		_expectedErr:      ErrItemNotFound,
		_expectedFound:    false,
		_expectedRejected: []ArchiveRow{fixtureRejected},

		id:         "B-999",
		resolution: "x",
		status:     StatusDone,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := writeStore(t, fixture)

			err := MoveToArchive(test.id, path, test.resolution, test.status)

			require.ErrorIs(t, err, test._expectedErr)
			store, loadErr := Load(path)
			require.NoError(t, loadErr)
			_, found := store.Find(test.id)
			assert.Equal(t, test._expectedFound, found)
			assert.Equal(t, test._expectedActive, store.Active())
			assert.Equal(t, test._expectedDelta, store.Changelog[len(store.Changelog)-1].Delta)
			assert.Equal(t, test._expectedDone, store.Archive.Done)
			assert.Equal(t, test._expectedRejected, store.Archive.Rejected)
		})
	}
}

func TestAddSection(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedErr     error

		name string
		path string
	}

	today := time.Now().Format(dateLayout)
	iceboxFixture := strings.Replace(fixture, `"name": "Railroad"`, `"name": "Icebox"`, 1)
	emptyBugs := "    {\n      \"items\": [],\n      \"name\": \"Bugs\"\n    },\n"
	iceboxStart := "    {\n      \"items\": [\n        {\n          \"id\": \"B-005\""
	tests := make([]*testCase, 0)

	// inserts-before-icebox
	insertedBeforeIcebox := strings.Replace(iceboxFixture, iceboxStart, emptyBugs+iceboxStart, 1)
	tests = append(tests, &testCase{
		_id:              "inserts-before-icebox",
		_expectedContent: withChangelogRow(insertedBeforeIcebox, "epic Bugs added (board)", today),
		_expectedErr:     nil,

		name: " Bugs ",
		path: writeStore(t, iceboxFixture),
	})

	// appends-without-icebox
	tests = append(tests, &testCase{
		_id:              "appends-without-icebox",
		_expectedContent: withChangelogRow(withEmptySection(fixture, "Bugs"), "epic Bugs added (board)", today),
		_expectedErr:     nil,

		name: "Bugs",
		path: writeStore(t, fixture),
	})

	// blank-refused
	tests = append(tests, &testCase{
		_id:              "blank-refused",
		_expectedContent: fixture,
		_expectedErr:     ErrInvalidSection,

		name: "  ",
		path: writeStore(t, fixture),
	})

	// icebox-refused
	tests = append(tests, &testCase{
		_id:              "icebox-refused",
		_expectedContent: fixture,
		_expectedErr:     ErrInvalidSection,

		name: IceboxSection,
		path: writeStore(t, fixture),
	})

	// duplicate-refused
	tests = append(tests, &testCase{
		_id:              "duplicate-refused",
		_expectedContent: fixture,
		_expectedErr:     ErrInvalidSection,

		name: "Defense",
		path: writeStore(t, fixture),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := AddSection(test.name, test.path)
			assert.ErrorIs(t, err, test._expectedErr)
			// byte-exact on purpose: the canonical form is the contract, not JSON equivalence
			assert.Equal(t, strings.Split(test._expectedContent, "\n"), strings.Split(readFile(t, test.path), "\n"))
		})
	}
}

func TestMoveItem(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedDelta    string
		_expectedErr      error
		_expectedSections string
		_isUntouched      bool

		id      string
		path    string
		section string
	}

	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	unchangedSections := "Defense: B-001,B-002 | Railroad: B-005"
	tests := make([]*testCase, 0)

	// moves-to-end-of-target
	tests = append(tests, &testCase{
		_id:               "moves-to-end-of-target",
		_expectedDelta:    "B-002 moved Defense → Railroad (board)",
		_expectedErr:      nil,
		_expectedSections: "Defense: B-001 | Railroad: B-005,B-002",
		_isUntouched:      false,

		id:      "B-002",
		path:    writeStaleStore(t, fixture),
		section: "Railroad",
	})

	// same-section-no-write
	tests = append(tests, &testCase{
		_id:               "same-section-no-write",
		_expectedDelta:    "Initial",
		_expectedErr:      nil,
		_expectedSections: unchangedSections,
		_isUntouched:      true,

		id:      "B-002",
		path:    writeStaleStore(t, fixture),
		section: "Defense",
	})

	// unknown-section
	tests = append(tests, &testCase{
		_id:               "unknown-section",
		_expectedDelta:    "Initial",
		_expectedErr:      ErrInvalidSection,
		_expectedSections: unchangedSections,
		_isUntouched:      true,

		id:      "B-002",
		path:    writeStaleStore(t, fixture),
		section: "Nope",
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:               "unknown-id",
		_expectedDelta:    "Initial",
		_expectedErr:      ErrItemNotFound,
		_expectedSections: unchangedSections,
		_isUntouched:      true,

		id:      "B-999",
		path:    writeStaleStore(t, fixture),
		section: "Railroad",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := MoveItem(test.id, test.path, test.section)
			assert.ErrorIs(t, err, test._expectedErr)

			info, statErr := os.Stat(test.path)
			require.NoError(t, statErr)
			assert.Equal(t, test._isUntouched, info.ModTime().Equal(epoch))
			assert.Equal(t, test._expectedSections, sectionSummary(test.path))
			store, loadErr := Load(test.path)
			require.NoError(t, loadErr)
			assert.Equal(t, test._expectedDelta, store.Changelog[len(store.Changelog)-1].Delta)
		})
	}
}

func TestRemoveSection(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedErr     error

		name string
		path string
	}

	today := time.Now().Format(dateLayout)
	withIcebox := withEmptySection(fixture, IceboxSection)
	tests := make([]*testCase, 0)

	// removes-empty
	tests = append(tests, &testCase{
		_id:              "removes-empty",
		_expectedContent: withChangelogRow(fixture, "epic Bugs removed (board)", today),
		_expectedErr:     nil,

		name: "Bugs",
		path: writeStore(t, withEmptySection(fixture, "Bugs")),
	})

	// icebox-refused
	tests = append(tests, &testCase{
		_id:              "icebox-refused",
		_expectedContent: withIcebox,
		_expectedErr:     ErrInvalidSection,

		name: IceboxSection,
		path: writeStore(t, withIcebox),
	})

	// non-empty-refused
	tests = append(tests, &testCase{
		_id:              "non-empty-refused",
		_expectedContent: fixture,
		_expectedErr:     ErrInvalidSection,

		name: "Defense",
		path: writeStore(t, fixture),
	})

	// unknown-refused
	tests = append(tests, &testCase{
		_id:              "unknown-refused",
		_expectedContent: fixture,
		_expectedErr:     ErrInvalidSection,

		name: "Nope",
		path: writeStore(t, fixture),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := RemoveSection(test.name, test.path)
			assert.ErrorIs(t, err, test._expectedErr)
			// byte-exact on purpose: the canonical form is the contract, not JSON equivalence
			assert.Equal(t, strings.Split(test._expectedContent, "\n"), strings.Split(readFile(t, test.path), "\n"))
		})
	}
}

func TestClampTitle(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		title string
	}
	tests := make([]*testCase, 0)

	// within-cap-unchanged
	tests = append(tests, &testCase{_id: "within-cap-unchanged", _expected: "Short title", title: "Short title"})

	// at-cap-unchanged
	tests = append(tests, &testCase{_id: "at-cap-unchanged", _expected: strings.Repeat("a", 120), title: strings.Repeat("a", 120)})

	// over-cap-ellipsis
	tests = append(tests, &testCase{_id: "over-cap-ellipsis", _expected: strings.Repeat("a", 119) + "…", title: strings.Repeat("a", 130)})

	// multibyte-runes
	tests = append(tests, &testCase{_id: "multibyte-runes", _expected: strings.Repeat("ä", 119) + "…", title: strings.Repeat("ä", 121)})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			clamped := ClampTitle(test.title)
			assert.Equal(t, test._expected, clamped)
			assert.LessOrEqual(t, utf8.RuneCountInString(clamped), TitleMaxRunes)
		})
	}
}

func TestItem_Validate(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedErr error
		_shouldPass  bool

		item *Item
	}
	tests := make([]*testCase, 0)

	// pass-title-120-runes
	atCap := provideIceboxItem()
	atCap.Title = strings.Repeat("ä", 120)
	tests = append(tests, &testCase{_id: "pass-title-120-runes", _expectedErr: nil, _shouldPass: true, item: atCap})

	// fail-title-121-runes
	overCap := provideIceboxItem()
	overCap.Title = strings.Repeat("ä", 121)
	tests = append(tests, &testCase{_id: "fail-title-121-runes", _expectedErr: ErrFormat, _shouldPass: false, item: overCap})

	// fail-blank-title
	blank := provideIceboxItem()
	blank.Title = "  "
	tests = append(tests, &testCase{_id: "fail-blank-title", _expectedErr: ErrFormat, _shouldPass: false, item: blank})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := test.item.Validate()
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.ErrorIs(t, err, test._expectedErr)
		})
	}
}

func TestStore_SectionNames(t *testing.T) {
	store, err := Load(writeStore(t, withEmptySection(fixture, IceboxSection)))
	require.NoError(t, err)

	assert.Equal(t, []string{"Defense", "Railroad", IceboxSection}, store.SectionNames())
}

// provideIceboxItem is an oversized change's row as the nightly files it, every field set.
func provideIceboxItem() *Item {
	item := &Item{
		Id:          "",
		Added:       "2026-10-08",
		Deps:        []string{},
		Description: "The problem.\nThe fix.\n8 files, 8 changed lines\nDone when: the change lands.",
		Evidence:    []string{"commit abc123", "s1"},
		Link:        "",
		Mined:       &Mined{Date: "2026-10-08", Sessions: []string{"s1"}},
		Pri:         "high",
		Route:       "/fdesign",
		Section:     "",
		Size:        "L",
		Status:      StatusShaped,
		Theme:       "smine",
		Title:       "skills.big: Changed eight files",
	}
	return item
}

// iceboxSummary renders the store as "<sections> sections | Icebox <ids> | <last delta>"; empty when it does not load.
func iceboxSummary(path string) string {
	store, err := Load(path)
	if err != nil {
		return ""
	}

	ids := make([]string, 0)
	for _, item := range store.Items() {
		if item.Section == IceboxSection {
			ids = append(ids, item.Id)
		}
	}
	return fmt.Sprintf("%d sections | Icebox %s | %s", len(store.Sections), strings.Join(ids, ","), store.Changelog[len(store.Changelog)-1].Delta)
}

func TestAppendIcebox(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedErr     error
		_expectedId      string
		_expectedSummary string

		item *Item
		path string
	}

	addedDelta := "B-100 added by the nightly: skills.big: Changed eight files"
	tests := make([]*testCase, 0)

	// appended-next-id
	tests = append(tests, &testCase{
		_id:              "appended-next-id",
		_expectedErr:     nil,
		_expectedId:      "B-100",
		_expectedSummary: "3 sections | Icebox B-100 | " + addedDelta,

		item: provideIceboxItem(),
		path: writeStore(t, fixture),
	})

	// existing-section-appended
	tests = append(tests, &testCase{
		_id:              "existing-section-appended",
		_expectedErr:     nil,
		_expectedId:      "B-100",
		_expectedSummary: "2 sections | Icebox B-005,B-100 | " + addedDelta,

		item: provideIceboxItem(),
		path: writeStore(t, strings.Replace(fixture, `"name": "Railroad"`, `"name": "Icebox"`, 1)),
	})

	// duplicate-evidence-returns-existing
	duplicatePath := writeStore(t, fixture)
	_, err := AppendIcebox(provideIceboxItem(), duplicatePath)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:              "duplicate-evidence-returns-existing",
		_expectedErr:     nil,
		_expectedId:      "B-100",
		_expectedSummary: "3 sections | Icebox B-100 | " + addedDelta,

		item: provideIceboxItem(),
		path: duplicatePath,
	})

	// invalid-item
	invalidItem := provideIceboxItem()
	invalidItem.Pri = "P1"
	tests = append(tests, &testCase{
		_id:              "invalid-item",
		_expectedErr:     ErrFormat,
		_expectedId:      "",
		_expectedSummary: "2 sections | Icebox  | Initial",

		item: invalidItem,
		path: writeStore(t, fixture),
	})

	// missing-store
	tests = append(tests, &testCase{
		_id:              "missing-store",
		_expectedErr:     os.ErrNotExist,
		_expectedId:      "",
		_expectedSummary: "",

		item: provideIceboxItem(),
		path: filepath.Join(t.TempDir(), "nope.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			id, err := AppendIcebox(test.item, test.path)
			assert.ErrorIs(t, err, test._expectedErr)
			assert.Equal(t, test._expectedId, id)
			assert.Equal(t, test._expectedSummary, iceboxSummary(test.path))
		})
	}
}

func TestAppendRejected(t *testing.T) {
	path := writeStore(t, fixture)
	row := &ArchiveRow{Commit: "abc", Description: "d", Link: "l", Reason: "wrong layer", Title: "t"}
	require.NoError(t, AppendRejected(path, row))

	store, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "B-100", store.Archive.Rejected[1].Id)
	assert.Equal(t, "abc", store.Archive.Rejected[1].Commit)
	assert.Equal(t, "B-100 proposal abc rejected (proposals): wrong layer", store.Changelog[len(store.Changelog)-1].Delta)
}

func TestSave_roundTrip(t *testing.T) {
	path := writeStore(t, fixture)
	store, err := Load(path)
	require.NoError(t, err)
	require.NoError(t, Save(path, store))

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	// byte-exact on purpose: the canonical form is the contract, not JSON equivalence
	assert.Equal(t, strings.Split(fixture, "\n"), strings.Split(string(after), "\n"))
}

func TestStore_NextId(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		store *Store
	}
	tests := make([]*testCase, 0)

	// empty-store
	tests = append(tests, &testCase{_id: "empty-store", _expected: "B-001", store: &Store{}})

	// archive-only
	archived := &Store{}
	archived.Archive.Rejected = []ArchiveRow{{Id: "B-041"}}
	tests = append(tests, &testCase{_id: "archive-only", _expected: "B-042", store: archived})

	// gap-in-ids
	gapped := &Store{Sections: []Section{{Items: []Item{{Id: "B-003"}, {Id: "B-010"}}}}}
	tests = append(tests, &testCase{_id: "gap-in-ids", _expected: "B-011", store: gapped})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.store.NextId())
		})
	}
}

func TestItem_RouteArgs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		route string
	}
	tests := make([]*testCase, 0)

	// change-route
	tests = append(tests, &testCase{_id: "change-route", _expected: "change", route: "/fdesign change"})

	// bare-route
	tests = append(tests, &testCase{_id: "bare-route", _expected: "", route: "/concept"})

	// multi-word-args
	tests = append(tests, &testCase{_id: "multi-word-args", _expected: "routine  x", route: "/skillroutine-create routine  x "})

	// direct
	tests = append(tests, &testCase{_id: "direct", _expected: "", route: "direct"})

	// empty
	tests = append(tests, &testCase{_id: "empty", _expected: "", route: ""})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			item := &Item{Route: test.route}
			assert.Equal(t, test._expected, item.RouteArgs())
		})
	}
}

func TestItem_RouteSkill(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		route string
	}
	tests := make([]*testCase, 0)

	// change-route
	tests = append(tests, &testCase{_id: "change-route", _expected: "fdesign", route: "/fdesign change"})

	// critical-arg
	tests = append(tests, &testCase{_id: "critical-arg", _expected: "idea", route: "/idea critical"})

	// direct
	tests = append(tests, &testCase{_id: "direct", _expected: "", route: "direct"})

	// empty
	tests = append(tests, &testCase{_id: "empty", _expected: "", route: ""})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			item := &Item{Route: test.route}
			assert.Equal(t, test._expected, item.RouteSkill())
		})
	}
}
