package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const (
	afterDenyFormat  = "skill-invoke-gate: context entries are injected via your skill invocation — do not read %s directly (language guides and, via Read, the files your lane manifest lists are the only exceptions)."
	beforeDenyFormat = "skill-invoke-gate: invoke the Skill tool with skill=\"%s\" before any other tool call — your review context arrives via that invocation."
	contextIndexJson = `{"entries": [], "tombstones": [], "aspects": [], "guides": [{"name": "go", "path": "rules/go.md", "files": ["*.go"], "source": "rules/go.md"}]}`
	markerText       = "REQUIRE-SKILL:railroad-review\nYou are lane 1 of a review."
	plainText        = "just do a normal thing please"
	relayText        = "[Workflow harness — user request] relayed"
	testSessionId    = "session-1"
	testSkill        = "railroad-review"
)

func TestRun(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedExit   int
		_expectedStderr string
		_expectedStdout string

		args   []string
		stdin  *strings.Reader
		stdout *bytes.Buffer
	}

	t.Setenv(contextIndexKey, "")
	root := t.TempDir()
	claudeHome := provideClaudeHome(t, root+"/claude")
	repo := provideRepo(t, root+"/repo")
	args := []string{"-claude-home", claudeHome}
	markerLine := provideUserLine(t, markerText)
	plainLine := provideUserLine(t, plainText)
	relayLine := provideUserLine(t, relayText)
	skillLine := provideSkillLine(t, testSkill)
	fdesignLine := provideSkillLine(t, "fdesign")
	markerOnly := provideTranscript(t, root+"/marker_only.jsonl", markerLine)
	noMarker := provideTranscript(t, root+"/no_marker.jsonl", plainLine)
	invoked := provideTranscript(t, root+"/invoked.jsonl", markerLine, skillLine)
	wrongSkill := provideTranscript(t, root+"/wrong_skill.jsonl", markerLine, fdesignLine)
	reviewing := repo + "/ctx/actions/reviewing.md"
	navigating := repo + "/ctx/actions/navigating.md"
	factsRepo := repo + "/ctx/facts/repo.md"
	goGuide := repo + "/ctx/rules/go.md"
	mainGo := repo + "/main.go"

	tests := make([]*testCase, 0)

	// no-marker-bash-silent
	call := provideBash(repo, "git status", noMarker)
	tests = append(tests, &testCase{
		_id:             "no-marker-bash-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// no-marker-read-silent
	call = provideRead(repo, goGuide, noMarker)
	tests = append(tests, &testCase{
		_id:             "no-marker-read-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pre-invocation-bash-denied
	call = provideBash(repo, "git rev-parse HEAD", markerOnly)
	tests = append(tests, &testCase{
		_id:             "pre-invocation-bash-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pre-invocation-skill-passes
	call = provideSkill(repo, markerOnly)
	tests = append(tests, &testCase{
		_id:             "pre-invocation-skill-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// invocation-satisfies
	call = provideBash(repo, "git status", invoked)
	tests = append(tests, &testCase{
		_id:             "invocation-satisfies",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// wrong-skill-does-not-satisfy
	call = provideBash(repo, "git status", wrongSkill)
	tests = append(tests, &testCase{
		_id:             "wrong-skill-does-not-satisfy",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// prompt-start-slash-counts
	slashLine := provideUserLine(t, "/railroad-review\nREQUIRE-SKILL:railroad-review")
	slashMarker := provideTranscript(t, root+"/slash_marker.jsonl", slashLine)
	call = provideBash(repo, "git status", slashMarker)
	tests = append(tests, &testCase{
		_id:             "prompt-start-slash-counts",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-entry-read-denied
	call = provideRead(repo, reviewing, invoked)
	tests = append(tests, &testCase{
		_id:             "context-entry-read-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-entry-cat-denied
	call = provideBash(repo, "cat "+reviewing, invoked)
	tests = append(tests, &testCase{
		_id:             "context-entry-cat-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// language-guide-read-allowed
	call = provideRead(repo, goGuide, invoked)
	tests = append(tests, &testCase{
		_id:             "language-guide-read-allowed",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// non-context-read-allowed
	call = provideRead(repo, mainGo, invoked)
	tests = append(tests, &testCase{
		_id:             "non-context-read-allowed",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// cwd-context-non-entry-denied
	agentsMd := repo + "/ctx/AGENTS.md"
	call = provideRead(repo, agentsMd, invoked)
	tests = append(tests, &testCase{
		_id:             "cwd-context-non-entry-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, agentsMd),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pinned-entry-read-denied
	laneReviewing := root + "/lanes/l1/ctx/actions/reviewing.md"
	laneGuide := root + "/lanes/l1/ctx/rules/go.md"
	laneAgentsMd := root + "/lanes/l1/ctx/AGENTS.md"
	provideFile(t, laneReviewing, "reviewing\n")
	provideFile(t, laneGuide, "go\n")
	provideFile(t, laneAgentsMd, "agents\n")
	call = provideRead(repo, laneReviewing, invoked)
	tests = append(tests, &testCase{
		_id:             "pinned-entry-read-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, laneReviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pinned-entry-cat-denied
	call = provideBash(repo, "cat "+laneReviewing, invoked)
	tests = append(tests, &testCase{
		_id:             "pinned-entry-cat-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, laneReviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pinned-guide-read-allowed
	call = provideRead(repo, laneGuide, invoked)
	tests = append(tests, &testCase{
		_id:             "pinned-guide-read-allowed",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pinned-non-entry-allowed
	call = provideRead(repo, laneAgentsMd, invoked)
	tests = append(tests, &testCase{
		_id:             "pinned-non-entry-allowed",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-pre-invocation-denied
	parentS1 := provideTranscript(t, root+"/proj/s1.jsonl", plainLine)
	provideTranscript(t, root+"/proj/s1/subagents/workflows/wf_1/agent-a1.jsonl", relayLine, markerLine)
	call = provideBash(repo, "git rev-parse HEAD", parentS1)
	call.AgentId = "a1"
	call.SessionId = "s1"
	tests = append(tests, &testCase{
		_id:             "subagent-pre-invocation-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-post-invocation-passes
	provideTranscript(
		t,
		root+"/proj/s1/subagents/workflows/wf_1/agent-a2.jsonl",
		relayLine,
		markerLine,
		skillLine,
	)
	call = provideBash(repo, "git status", parentS1)
	call.AgentId = "a2"
	call.SessionId = "s1"
	tests = append(tests, &testCase{
		_id:             "subagent-post-invocation-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-missing-transcript-silent
	parentS2 := provideTranscript(t, root+"/proj/s2.jsonl", markerLine)
	call = provideBash(repo, "git status", parentS2)
	call.AgentId = "nope"
	call.SessionId = "s2"
	tests = append(tests, &testCase{
		_id:             "subagent-missing-transcript-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// marker-after-relay-line-denied
	relay := provideTranscript(t, root+"/relay.jsonl", relayLine, markerLine)
	call = provideBash(repo, "git rev-parse HEAD", relay)
	tests = append(tests, &testCase{
		_id:             "marker-after-relay-line-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// marker-after-first-assistant-ignored
	late := provideTranscript(t, root+"/late.jsonl", plainLine, fdesignLine, markerLine)
	call = provideBash(repo, "git rev-parse HEAD", late)
	tests = append(tests, &testCase{
		_id:             "marker-after-first-assistant-ignored",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-own-read-passes
	requiredS3 := claudeHome + "/context-required/s3/global.md"
	requiredS4 := claudeHome + "/context-required/s4/global.md"
	provideFile(t, requiredS3, "global\n")
	provideFile(t, requiredS4, "other\n")
	call = provideRead(repo, requiredS3, markerOnly)
	call.SessionId = "s3"
	tests = append(tests, &testCase{
		_id:             "context-required-own-read-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-outside-read-denied
	call = provideRead(repo, mainGo, markerOnly)
	call.SessionId = "s3"
	tests = append(tests, &testCase{
		_id:             "context-required-outside-read-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-other-session-denied
	call = provideRead(repo, requiredS4, markerOnly)
	call.SessionId = "s3"
	tests = append(tests, &testCase{
		_id:             "context-required-other-session-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-dot-segment-denied
	call = provideRead(repo, claudeHome+"/context-required/s3/../s4/global.md", markerOnly)
	call.SessionId = "s3"
	tests = append(tests, &testCase{
		_id:             "context-required-dot-segment-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-missing-session-denied
	call = provideRead(repo, requiredS3, markerOnly)
	tests = append(tests, &testCase{
		_id:             "context-required-missing-session-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-dot-session-id-denied
	call = provideRead(repo, requiredS4, markerOnly)
	call.SessionId = "s3/.."
	tests = append(tests, &testCase{
		_id:             "context-required-dot-session-id-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-cat-denied
	call = provideBash(repo, "cat "+requiredS3, markerOnly)
	call.SessionId = "s3"
	tests = append(tests, &testCase{
		_id:             "context-required-cat-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-post-invocation-passes
	call = provideRead(repo, requiredS3, invoked)
	call.SessionId = "s3"
	tests = append(tests, &testCase{
		_id:             "context-required-post-invocation-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-subagent-read-passes
	requiredS5 := claudeHome + "/context-required/s5/global.md"
	provideFile(t, requiredS5, "global\n")
	parentS5 := provideTranscript(t, root+"/proj/s5.jsonl", plainLine)
	provideTranscript(t, root+"/proj/s5/subagents/workflows/wf_5/agent-a5.jsonl", relayLine, markerLine)
	call = provideRead(repo, requiredS5, parentS5)
	call.AgentId = "a5"
	call.SessionId = "s5"
	tests = append(tests, &testCase{
		_id:             "context-required-subagent-read-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-required-subagent-bash-denied
	call = provideBash(repo, "git rev-parse HEAD", parentS5)
	call.AgentId = "a5"
	call.SessionId = "s5"
	tests = append(tests, &testCase{
		_id:             "context-required-subagent-bash-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-listed-read-passes
	manifestOne := root + "/round-1/critical-lane1.json.lane-manifest.json"
	manifestTwo := root + "/round-1/critical-lane2.json.lane-manifest.json"
	provideFile(t, manifestOne, `{"artifact": "a.json", "direction": "critical", "files": ["main.go", "ctx/actions/reviewing.md"], "partners": [], "root": "", "walk": null}`)
	provideFile(t, manifestTwo, `{"artifact": "b.json", "direction": "critical", "files": ["ctx/actions/navigating.md"], "partners": [], "root": "", "walk": null}`)
	indentedLine := provideUserLine(t, "  REQUIRE-SKILL:railroad-review\n  LANE-MANIFEST:"+manifestOne+"\n  You are lane 1 of a review.")
	oneManifest := provideTranscript(t, root+"/manifest_one.jsonl", relayLine, indentedLine, skillLine)
	call = provideRead(repo, reviewing, oneManifest)
	tests = append(tests, &testCase{
		_id:             "manifest-listed-read-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-unlisted-read-denied
	call = provideRead(repo, factsRepo, oneManifest)
	tests = append(tests, &testCase{
		_id:             "manifest-unlisted-read-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, factsRepo),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-listed-cat-denied
	call = provideBash(repo, "cat "+reviewing, oneManifest)
	tests = append(tests, &testCase{
		_id:             "manifest-listed-cat-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-dot-segment-denied
	dotted := repo + "/ctx/facts/../actions/reviewing.md"
	call = provideRead(repo, dotted, oneManifest)
	tests = append(tests, &testCase{
		_id:             "manifest-dot-segment-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, dotted),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-guide-passes
	call = provideRead(repo, goGuide, oneManifest)
	tests = append(tests, &testCase{
		_id:             "manifest-guide-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// two-manifests-second-listed-passes
	twoManifests := provideMarkerTranscript(t, root+"/manifest_two.jsonl", "LANE-MANIFEST:"+manifestOne+"\nLANE-MANIFEST:"+manifestTwo)
	call = provideRead(repo, navigating, twoManifests)
	tests = append(tests, &testCase{
		_id:             "two-manifests-second-listed-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// two-manifests-first-listed-passes
	call = provideRead(repo, reviewing, twoManifests)
	tests = append(tests, &testCase{
		_id:             "two-manifests-first-listed-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// two-manifests-unlisted-denied
	call = provideRead(repo, factsRepo, twoManifests)
	tests = append(tests, &testCase{
		_id:             "two-manifests-unlisted-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, factsRepo),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-before-invocation-denied
	preLine := provideUserLine(t, "REQUIRE-SKILL:railroad-review\nLANE-MANIFEST:"+manifestOne)
	manifestPre := provideTranscript(t, root+"/manifest_pre.jsonl", preLine)
	call = provideRead(repo, reviewing, manifestPre)
	tests = append(tests, &testCase{
		_id:             "manifest-before-invocation-denied",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-root-listed-passes
	rootLane := root + "/wt/critical-lane3"
	rootLaneReviewing := rootLane + "/ctx/actions/reviewing.md"
	manifestThree := root + "/round-1/critical-lane3.json.lane-manifest.json"
	provideFile(t, rootLaneReviewing, "reviewing\n")
	provideFile(t, manifestThree, `{"artifact": "c.json", "direction": "critical", "files": ["ctx/actions/reviewing.md"], "partners": [], "root": "`+rootLane+`/", "walk": null}`)
	manifestRoot := provideMarkerTranscript(t, root+"/manifest_root.jsonl", "LANE-MANIFEST:"+manifestThree)
	call = provideRead(repo, rootLaneReviewing, manifestRoot)
	tests = append(tests, &testCase{
		_id:             "manifest-root-listed-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// manifest-root-wins-over-cwd
	call = provideRead(repo, reviewing, manifestRoot)
	tests = append(tests, &testCase{
		_id:             "manifest-root-wins-over-cwd",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// unreadable-manifest-absent-denied
	roundTwo := root + "/round-2"
	locked := roundTwo + "/locked.lane-manifest.json"
	broken := roundTwo + "/broken.lane-manifest.json"
	good := roundTwo + "/good.lane-manifest.json"
	provideFile(t, locked, `{"files": ["ctx/actions/reviewing.md"], "root": ""}`)
	require.NoError(t, os.Chmod(locked, 0o000))
	provideFile(t, broken, "not json\n")
	provideFile(t, good, `{"files": ["ctx/actions/navigating.md"], "root": ""}`)
	absentManifest := provideMarkerTranscript(t, root+"/unreadable_absent.jsonl", "LANE-MANIFEST:"+roundTwo+"/absent.lane-manifest.json")
	call = provideRead(repo, reviewing, absentManifest)
	tests = append(tests, &testCase{
		_id:             "unreadable-manifest-absent-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// unreadable-manifest-locked-denied
	lockedManifest := provideMarkerTranscript(t, root+"/unreadable_locked.jsonl", "LANE-MANIFEST:"+locked)
	call = provideRead(repo, reviewing, lockedManifest)
	tests = append(tests, &testCase{
		_id:             "unreadable-manifest-locked-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// unreadable-manifest-not-json-denied
	brokenManifest := provideMarkerTranscript(t, root+"/unreadable_broken.jsonl", "LANE-MANIFEST:"+broken)
	call = provideRead(repo, reviewing, brokenManifest)
	tests = append(tests, &testCase{
		_id:             "unreadable-manifest-not-json-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// unreadable-manifest-directory-denied
	directoryManifest := provideMarkerTranscript(t, root+"/unreadable_directory.jsonl", "LANE-MANIFEST:"+roundTwo)
	call = provideRead(repo, reviewing, directoryManifest)
	tests = append(tests, &testCase{
		_id:             "unreadable-manifest-directory-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// malformed-marker-empty-path-denied
	emptyMarker := provideMarkerTranscript(t, root+"/malformed_empty.jsonl", "LANE-MANIFEST:")
	call = provideRead(repo, navigating, emptyMarker)
	tests = append(tests, &testCase{
		_id:             "malformed-marker-empty-path-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, navigating),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// malformed-marker-space-before-path-denied
	spacedMarker := provideMarkerTranscript(t, root+"/malformed_spaced.jsonl", "LANE-MANIFEST: "+good)
	call = provideRead(repo, navigating, spacedMarker)
	tests = append(tests, &testCase{
		_id:             "malformed-marker-space-before-path-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, navigating),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// malformed-marker-relative-path-denied
	relativeMarker := provideMarkerTranscript(t, root+"/malformed_relative.jsonl", "LANE-MANIFEST:round-2/good.lane-manifest.json")
	call = provideRead(repo, navigating, relativeMarker)
	tests = append(tests, &testCase{
		_id:             "malformed-marker-relative-path-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, navigating),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// malformed-marker-no-hyphen-denied
	unhyphenatedMarker := provideMarkerTranscript(t, root+"/malformed_unhyphenated.jsonl", "LANE MANIFEST:"+good)
	call = provideRead(repo, navigating, unhyphenatedMarker)
	tests = append(tests, &testCase{
		_id:             "malformed-marker-no-hyphen-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, navigating),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// readable-manifest-beside-locked-passes
	partial := provideMarkerTranscript(t, root+"/partial.jsonl", "LANE-MANIFEST:"+locked+"\nLANE-MANIFEST:"+good)
	call = provideRead(repo, navigating, partial)
	tests = append(tests, &testCase{
		_id:             "readable-manifest-beside-locked-passes",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// listed-only-in-locked-manifest-denied
	call = provideRead(repo, reviewing, partial)
	tests = append(tests, &testCase{
		_id:             "listed-only-in-locked-manifest-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, reviewing),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// absent-transcript-silent
	call = provideBash(repo, "git status", root+"/absent.jsonl")
	tests = append(tests, &testCase{
		_id:             "absent-transcript-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// empty-transcript-path-silent
	call = provideBash(repo, "git status", "")
	tests = append(tests, &testCase{
		_id:             "empty-transcript-path-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// empty-context-dir-allows
	noContextHome := root + "/no-context"
	writeHookConfig(t, noContextHome, provideHookConfig(t, ""))
	call = provideRead(repo, reviewing, invoked)
	tests = append(tests, &testCase{
		_id:             "empty-context-dir-allows",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   []string{"-claude-home", noContextHome},
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// deny-text-from-config
	customHome := root + "/custom-text"
	customConfig := provideHookConfig(t, "ctx")
	customConfig.Skillinvokegate.DenyTexts[1] = "custom: invoke %s first"
	writeHookConfig(t, customHome, customConfig)
	call = provideBash(repo, "git status", markerOnly)
	tests = append(tests, &testCase{
		_id:             "deny-text-from-config",
		_expectedExit:   0,
		_expectedStdout: provideVerdict(t, "custom: invoke railroad-review first"),

		args:   []string{"-claude-home", customHome},
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// missing-config-unmarked-silent
	missingHome := root + "/missing-config"
	call = provideBash(repo, "git status", noMarker)
	tests = append(tests, &testCase{
		_id:             "missing-config-unmarked-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   []string{"-claude-home", missingHome},
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// undecodable-transcript-line-silent
	undecodable := provideTranscript(t, root+"/undecodable.jsonl", markerLine, "{not json")
	call = provideBash(repo, "git status", undecodable)
	tests = append(tests, &testCase{
		_id:             "undecodable-transcript-line-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// mistyped-transcript-field-still-judged
	mistypedLine := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/x","limit":"100"}}]}}`
	mistyped := provideTranscript(t, root+"/mistyped.jsonl", markerLine, mistypedLine)
	call = provideBash(repo, "git status", mistyped)
	tests = append(tests, &testCase{
		_id:             "mistyped-transcript-field-still-judged",
		_expectedExit:   0,
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// empty-marker-name-silent
	emptyNameLine := provideUserLine(t, "REQUIRE-SKILL:\nREQUIRE-SKILL:railroad-review")
	emptyName := provideTranscript(t, root+"/empty_name.jsonl", emptyNameLine)
	call = provideBash(repo, "git status", emptyName)
	tests = append(tests, &testCase{
		_id:             "empty-marker-name-silent",
		_expectedExit:   0,
		_expectedStdout: "",

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// cat-glob-word-denied
	globWord := repo + "/ctx/actions/*.md"
	call = provideBash(repo, "cat "+globWord, invoked)
	tests = append(tests, &testCase{
		_id:             "cat-glob-word-denied",
		_expectedExit:   0,
		_expectedStdout: provideAfterDeny(t, globWord),

		args:   args,
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// malformed-input-exits-1
	tests = append(tests, &testCase{
		_id:             "malformed-input-exits-1",
		_expectedExit:   1,
		_expectedStdout: "",

		args:   args,
		stdin:  strings.NewReader("{not json"),
		stdout: &bytes.Buffer{},
	})

	// switch-off-allows
	switchHome := provideClaudeHome(t, root+"/switch-off")
	provideSwitchesOff(t, switchHome, sessionstate.SkillInvokegate)
	call = &HookInput{Cwd: repo, SessionId: testSessionId, ToolInput: ToolInput{Command: "git status"}, ToolName: toolBash, TranscriptPath: markerOnly}
	tests = append(tests, &testCase{
		_id:             "switch-off-allows",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		args:   []string{"-claude-home", switchHome},
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// record-garbage-denies-and-logs
	garbageHome := provideClaudeHome(t, root+"/garbage")
	garbage := provideGarbageRecord(t, garbageHome)
	call = &HookInput{Cwd: repo, SessionId: testSessionId, ToolInput: ToolInput{Command: "git status"}, ToolName: toolBash, TranscriptPath: markerOnly}
	tests = append(tests, &testCase{
		_id:             "record-garbage-denies-and-logs",
		_expectedExit:   0,
		_expectedStderr: fmt.Sprintf("skillinvokegate: isOff: IsOff: Load: Failed to decode %s: unexpected end of JSON input\n", garbage),
		_expectedStdout: provideBeforeDeny(t, testSkill),

		args:   []string{"-claude-home", garbageHome},
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stderr := provideStderr(t)
			exitCode := run(test.args, io.Discard, test.stdin, test.stdout)

			assert.Equal(t, test._expectedExit, exitCode)
			assert.Equal(t, test._expectedStderr, readStderr(t, stderr))
			assert.Equal(t, test._expectedStdout, test.stdout.String())
		})
	}
}

func TestRun_ContextIndexOverride(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedStdout string

		stdin  *strings.Reader
		stdout *bytes.Buffer
	}

	root := t.TempDir()
	claudeHome := provideClaudeHome(t, root+"/claude")
	repo := provideRepo(t, root+"/repo")
	args := []string{"-claude-home", claudeHome}
	nowhere := root + "/nowhere"
	require.NoError(t, os.MkdirAll(nowhere, 0o755))
	t.Setenv(contextIndexKey, repo+"/ctx/context.json")
	markerLine := provideUserLine(t, markerText)
	skillLine := provideSkillLine(t, testSkill)
	invoked := provideTranscript(t, root+"/invoked.jsonl", markerLine, skillLine)
	laneReviewing := root + "/lanes/l1/ctx/actions/reviewing.md"
	laneGuide := root + "/lanes/l1/ctx/rules/go.md"
	provideFile(t, laneReviewing, "reviewing\n")
	provideFile(t, laneGuide, "go\n")

	tests := make([]*testCase, 0)

	// override-index-entry-denied
	call := provideRead(nowhere, laneReviewing, invoked)
	tests = append(tests, &testCase{
		_id:             "override-index-entry-denied",
		_expectedStdout: provideAfterDeny(t, laneReviewing),

		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// override-index-guide-allowed
	call = provideRead(nowhere, laneGuide, invoked)
	tests = append(tests, &testCase{
		_id:             "override-index-guide-allowed",
		_expectedStdout: "",

		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exitCode := run(args, io.Discard, test.stdin, test.stdout)

			assert.Equal(t, 0, exitCode)
			assert.Equal(t, test._expectedStdout, test.stdout.String())
		})
	}
}

func TestRun_MissingConfigDenies(t *testing.T) {
	root := t.TempDir()
	claudeHome := root + "/claude"
	repo := provideRepo(t, root+"/repo")
	args := []string{"-claude-home", claudeHome}
	markerLine := provideUserLine(t, markerText)
	markerOnly := provideTranscript(t, root+"/marker_only.jsonl", markerLine)
	call := provideBash(repo, "git status", markerOnly)
	stdin := provideEnvelope(t, call)
	stdout := &bytes.Buffer{}

	exitCode := run(args, io.Discard, stdin, stdout)

	output := &HookOutput{}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), output))
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, decisionDeny, output.HookSpecificOutput.PermissionDecision)
	assert.Contains(t, output.HookSpecificOutput.PermissionDecisionReason, hookconfig.Path(claudeHome))
}

func provideAfterDeny(t *testing.T, target string) string {
	reason := fmt.Sprintf(afterDenyFormat, target)
	return provideVerdict(t, reason)
}

func provideBash(cwd string, command string, transcriptPath string) *HookInput {
	input := &HookInput{
		Cwd:            cwd,
		ToolInput:      ToolInput{Command: command},
		ToolName:       toolBash,
		TranscriptPath: transcriptPath,
	}
	return input
}

func provideBeforeDeny(t *testing.T, skill string) string {
	reason := fmt.Sprintf(beforeDenyFormat, skill)
	return provideVerdict(t, reason)
}

// provideClaudeHome builds a Claude home whose hooks config names "ctx" as the context dir.
func provideClaudeHome(t *testing.T, dir string) string {
	writeHookConfig(t, dir, provideHookConfig(t, "ctx"))
	return dir
}

func provideEnvelope(t *testing.T, input *HookInput) *strings.Reader {
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return strings.NewReader(string(data))
}

func provideFile(t *testing.T, path string, content string) {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
}

// provideGarbageRecord writes an undecodable switch record for the test session and returns its path.
func provideGarbageRecord(t *testing.T, claudeHome string) string {
	path := sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId)
	provideFile(t, path, "{")
	return path
}

// provideMarkerTranscript writes an invoked session whose prompt carries the
// REQUIRE-SKILL marker above the given marker lines.
// provideHookConfig is the gate's rendered defaults with contextDir as the context dir.
func provideHookConfig(t *testing.T, contextDir string) *hookconfig.Config {
	defaults, err := install.RenderDefaults(components.Gate, components.SinkHooksConfig)
	require.NoError(t, err)

	cfg := &hookconfig.Config{}
	require.NoError(t, json.Unmarshal(defaults, cfg))
	cfg.Context.ContextDir = contextDir
	return cfg
}

func provideMarkerTranscript(t *testing.T, path string, markers string) string {
	promptLine := provideUserLine(t, "REQUIRE-SKILL:railroad-review\n"+markers)
	skillLine := provideSkillLine(t, testSkill)
	return provideTranscript(t, path, promptLine, skillLine)
}

func provideRead(cwd string, filePath string, transcriptPath string) *HookInput {
	input := &HookInput{
		Cwd:            cwd,
		ToolInput:      ToolInput{FilePath: filePath},
		ToolName:       toolRead,
		TranscriptPath: transcriptPath,
	}
	return input
}

// provideRepo builds a repo whose ctx dir holds a Go guide indexed in
// context.json, two action entries and a fact entry.
func provideRepo(t *testing.T, dir string) string {
	provideFile(t, dir+"/ctx/rules/go.md", "go\n")
	provideFile(t, dir+"/ctx/context.json", contextIndexJson)
	provideFile(t, dir+"/ctx/actions/reviewing.md", "reviewing\n")
	provideFile(t, dir+"/ctx/actions/navigating.md", "navigating\n")
	provideFile(t, dir+"/ctx/facts/repo.md", "repo\n")
	return dir
}

func provideSkill(cwd string, transcriptPath string) *HookInput {
	input := &HookInput{
		Cwd:            cwd,
		ToolName:       toolSkill,
		TranscriptPath: transcriptPath,
	}
	return input
}

// provideSkillLine is an assistant line invoking the skill via the Skill
// tool.
func provideSkillLine(t *testing.T, skill string) string {
	name, err := json.Marshal(skill)
	require.NoError(t, err)
	return `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"s","name":"Skill","input":{"skill":` + string(name) + `}}]}}`
}

// provideStderr points os.Stderr, where isOff logs, at a file for the test's duration.
func provideStderr(t *testing.T) *os.File {
	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	require.NoError(t, err)

	original := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() {
		os.Stderr = original
		stderr.Close()
	})
	return stderr
}

// provideSwitchesOff writes the test session's switch record with the given features off.
func provideSwitchesOff(t *testing.T, claudeHome string, features ...sessionstate.Feature) {
	state := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: features}}
	err := sessionstate.Save(sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId), state)
	require.NoError(t, err)
}

func provideTranscript(t *testing.T, path string, lines ...string) string {
	content := strings.Join(lines, "\n") + "\n"
	provideFile(t, path, content)
	return path
}

// provideUserLine is a user line carrying the text as one text block.
func provideUserLine(t *testing.T, text string) string {
	body, err := json.Marshal(text)
	require.NoError(t, err)
	return `{"type":"user","message":{"content":[{"type":"text","text":` + string(body) + `}]}}`
}

func provideVerdict(t *testing.T, reason string) string {
	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       decisionDeny,
			PermissionDecisionReason: reason,
		},
	}
	data, err := json.Marshal(output)
	require.NoError(t, err)
	return string(data) + "\n"
}

func readStderr(t *testing.T, stderr *os.File) string {
	data, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	return string(data)
}

func writeHookConfig(t *testing.T, home string, cfg *hookconfig.Config) {
	encoded, err := json.Marshal(cfg)
	require.NoError(t, err)
	provideFile(t, hookconfig.Path(home), string(encoded))
}
