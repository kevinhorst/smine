//go:build acdslsmoke

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The smoke projects, stages and strips real tracked files of this checkout, so it runs alone: make test-acdsl-smoke.
// The marker and the directive are split so this file never trips the leak guards itself.
const (
	smokeMarker   = "[ACDSL-" + "PROJECTION]"
	smokeTarget   = "internal/routines/launchctl_darwin.go"
	smokeShell    = "install.sh"
	smokeProse    = "plans/agentic_context_dsl/reviews/smoke_prose.md"
	smokeHTML     = "plans/agentic_context_dsl/reviews/smoke_htmlblock.md"
	smokeFront    = "plans/agentic_context_dsl/design/smoke_frontmatter.md"
	smokeGateOnly = "internal/gateonlysmoke"
	smokeTask     = "xptask_smoke.acdsl"
)

var (
	rulesOK     = regexp.MustCompile(`(?m)^acdsl: (\d+) rule\(s\) OK$`)
	anyRulesOK  = regexp.MustCompile(`(\d+) rule\(s\) OK`)
	fixturesOK  = regexp.MustCompile(`fixtures OK \((\d+) rule\(s\) with examples`)
	fixturesSkp = regexp.MustCompile(` (\d+) skipped \(needs\)`)
)

type smoke struct {
	t     *testing.T
	ctx   context.Context
	root  string
	acdsl string
}

// run executes bin/acdsl at the repo root with deliberate reds kept out of the verdict sink; extra env rows override.
func (s *smoke) run(env []string, args ...string) (string, int) {
	s.t.Helper()
	cmd := exec.CommandContext(s.ctx, s.acdsl, args...)
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "ACDSL_VERDICTS_ENABLED=0")
	cmd.Env = append(cmd.Env, env...)
	output, err := cmd.CombinedOutput()
	s.t.Log(string(output))
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(output), exit.ExitCode()
	}
	require.NoError(s.t, err)
	return string(output), 0
}

// git runs git at the repo root and returns its combined output and exit code.
func (s *smoke) git(args ...string) (string, int) {
	s.t.Helper()
	cmd := exec.CommandContext(s.ctx, "git", args...)
	cmd.Dir = s.root
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(output), exit.ExitCode()
	}
	require.NoError(s.t, err)
	return string(output), 0
}

func (s *smoke) banner(text string) {
	s.t.Log("########## acdsl smoke: " + text)
}

func (s *smoke) path(rel string) string {
	return filepath.Join(s.root, filepath.FromSlash(rel))
}

func (s *smoke) write(rel, content string) {
	s.t.Helper()
	require.NoError(s.t, os.MkdirAll(filepath.Dir(s.path(rel)), 0o755))
	require.NoError(s.t, os.WriteFile(s.path(rel), []byte(content), 0o644))
}

func (s *smoke) lines(rel string) []string {
	s.t.Helper()
	content, err := os.ReadFile(s.path(rel))
	require.NoError(s.t, err)
	return strings.Split(string(content), "\n")
}

func (s *smoke) count(re *regexp.Regexp, text string) int {
	s.t.Helper()
	match := re.FindStringSubmatch(text)
	require.NotNil(s.t, match, "no %s in output", re)
	var n int
	_, err := fmt.Sscan(match[1], &n)
	require.NoError(s.t, err)
	return n
}

// cleanup runs after the test's context is cancelled, so it brings its own.
func (s *smoke) cleanup() {
	s.ctx = context.Background()
	_, _ = s.git("restore", "--staged", smokeTarget, smokeProse, smokeHTML)
	for _, rel := range []string{"internal/xptask", smokeTask, smokeTask + ".off", smokeGateOnly, smokeProse, smokeHTML, smokeFront, "internal/acdsl_smoke_untracked.go"} {
		_ = os.RemoveAll(s.path(rel))
	}
	_, _ = s.run(nil, "project", "-strip")
}

// build links bin/acdsl and bin/verifiers/* like make audit does; the registry swaps the binaries in for the go-run argv.
func (s *smoke) build() {
	s.t.Helper()
	require.NoError(s.t, os.MkdirAll(filepath.Join(s.root, "bin", "verifiers"), 0o755))
	for _, args := range [][]string{
		{"build", "-o", s.acdsl, "./cmd/acdsl"},
		{"build", "-o", filepath.Join(s.root, "bin", "verifiers"), "./cmd/acdsl/verifiers/..."},
	} {
		cmd := exec.CommandContext(s.ctx, "go", args...)
		cmd.Dir = s.root
		output, err := cmd.CombinedOutput()
		require.NoError(s.t, err, string(output))
	}
}

func TestSmoke(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	s := &smoke{t: t, ctx: t.Context(), root: root, acdsl: filepath.Join(root, "bin", "acdsl")}
	s.build()
	t.Cleanup(s.cleanup)

	// the working tree legitimately carries projections (any agent Read projects on disk); normalize first
	s.run(nil, "project", "-strip")

	s.banner("check on real tree (expect: all doctrine rules OK)")
	checkOut, code := s.run(nil, "check")
	require.Equal(t, 0, code)
	doctrine := s.count(rulesOK, checkOut)

	s.banner("fixtures prove all rules with examples (expect: every testdata dir checked or needs-skipped)")
	fixturesOut, code := s.run(nil, "fixtures")
	require.Equal(t, 0, code)
	entries, err := os.ReadDir(s.path("acdsl/testdata"))
	require.NoError(t, err)
	dirs := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirs++
		}
	}
	skipped := 0
	if fixturesSkp.MatchString(fixturesOut) {
		skipped = s.count(fixturesSkp, fixturesOut)
	}
	assert.Equal(t, dirs, s.count(fixturesOK, fixturesOut)+skipped)

	s.banner("project " + smokeTarget + " in place (expect: projected, 7 rules)")
	projectOut, _ := s.run(nil, "project", "-file", smokeTarget)
	assert.Contains(t, projectOut, "projected "+smokeTarget+" (7 rule(s))")

	s.banner("the projected file on disk, head (view == disk)")
	lines := s.lines(smokeTarget)
	assert.Contains(t, lines[0], smokeMarker)
	assert.Contains(t, strings.Join(lines, "\n"), "ACDSL-GOLANG-EXEC-001")

	s.banner("dirty set is the touched-files trace (expect: " + smokeTarget + " modified)")
	status, _ := s.git("status", "--short", smokeTarget)
	assert.True(t, strings.HasPrefix(status, " M"), status)

	s.banner("projection is idempotent (expect: already current)")
	idemOut, _ := s.run(nil, "project", "-file", smokeTarget)
	assert.Contains(t, idemOut, "already current")

	s.banner("staged projection is refused by the gate (expect: red)")
	s.git("add", smokeTarget)
	stagedOut, code := s.run(nil, "guard")
	assert.NotEqual(t, 0, code, "staged projection block not refused")
	assert.Contains(t, stagedOut, "projection content staged")
	s.git("restore", "--staged", smokeTarget)

	s.banner("staged doc quoting the marker is NOT refused (expect: green)")
	s.write(smokeProse, "# Smoke\nThe projection marker is "+smokeMarker+", quoted in prose.\n")
	s.git("add", smokeProse)
	_, code = s.run(nil, "guard")
	assert.Equal(t, 0, code, "prose doc quoting the marker tripped the guard")
	s.git("restore", "--staged", smokeProse)
	require.NoError(t, os.Remove(s.path(smokeProse)))

	s.banner("multi-syntax: shell keeps its shebang on line 1 (expect: block at line 2)")
	shellOut, _ := s.run(nil, "project", "-file", smokeShell)
	assert.Contains(t, shellOut, "projected "+smokeShell)
	lines = s.lines(smokeShell)
	assert.True(t, strings.HasPrefix(lines[0], "#!"), lines[0])
	assert.Contains(t, lines[1], "# "+smokeMarker)

	s.banner("multi-syntax: a frontmatter .md keeps its frontmatter on top (expect: block after the fence)")
	s.write(smokeFront, "---\nname: smoke\n---\n\n# Smoke\n")
	frontOut, _ := s.run(nil, "project", "-file", smokeFront)
	assert.Contains(t, frontOut, "projected "+smokeFront)
	lines = s.lines(smokeFront)
	assert.Equal(t, "---", lines[0])
	fenceEnd := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			fenceEnd = i
			break
		}
	}
	require.NotZero(t, fenceEnd, "no closing fence")
	assert.Contains(t, lines[fenceEnd+1], "<!-- "+smokeMarker)
	require.NoError(t, os.Remove(s.path(smokeFront)))

	s.banner("staged HTML-shaped block in a .md is refused (expect: red)")
	s.write(smokeHTML, "<!-- "+smokeMarker+" 1 rule(s) govern this file — working-copy view, stripped before commit -->\n# Smoke\n")
	s.git("add", smokeHTML)
	htmlOut, code := s.run(nil, "guard")
	assert.NotEqual(t, 0, code, "staged .md projection block not refused")
	assert.Contains(t, htmlOut, "smoke_htmlblock.md")
	s.git("restore", "--staged", smokeHTML)
	require.NoError(t, os.Remove(s.path(smokeHTML)))

	s.banner("context delivery for every governed file (expect: JSON-003 for json, EXEC-001 for go)")
	jsonCtx, _ := s.run(nil, "project", "-context", "acdsl/policy.json")
	assert.Contains(t, jsonCtx, "ACDSL-JSON-003")
	goCtx, _ := s.run(nil, "project", "-context", "internal/acdsl/project.go")
	assert.Contains(t, goCtx, "ACDSL-GOLANG-EXEC-001")
	_, code = s.git("diff", "--quiet", "--", "internal/acdsl/project.go")
	assert.Equal(t, 0, code, "a context read changed the file on disk")

	s.banner("strip restores original bytes (expect: 2 files, clean diff)")
	stripOut, _ := s.run(nil, "project", "-strip")
	assert.Contains(t, stripOut, "stripped 2 file(s)")
	_, code = s.git("diff", "--quiet", smokeTarget, smokeShell)
	assert.Equal(t, 0, code, "strip left a diff")

	s.banner("plan-time resolution for a future file (expect: EXEC-001 would govern)")
	planOut, _ := s.run(nil, "project", "-plan", "internal/brandnew/controller.go")
	assert.Contains(t, planOut, "ACDSL-GOLANG-EXEC-001")
	planNone, _ := s.run(nil, "project", "-plan", "docs/note.md")
	assert.Contains(t, planNone, "no rules would govern")

	s.banner("ungoverned file README.md (expect: no rules, untouched)")
	readmeBefore, err := os.ReadFile(s.path("README.md"))
	require.NoError(t, err)
	ungovOut, _ := s.run(nil, "project", "-file", "README.md")
	assert.Contains(t, ungovOut, "no rules govern README.md")
	readmeAfter, err := os.ReadFile(s.path("README.md"))
	require.NoError(t, err)
	assert.Equal(t, readmeBefore, readmeAfter)

	s.banner("untracked violation is caught without git add (expect: red, EXEC-001)")
	s.write("internal/acdsl_smoke_untracked.go", "package internal\n\nimport \"os/exec\"\n\nfunc smokeViolation() {\n\t_ = exec.Command(\"ls\")\n}\n")
	untrackedOut, code := s.run(nil, "check", "-rule", "ACDSL-GOLANG-EXEC-001")
	require.NoError(t, os.Remove(s.path("internal/acdsl_smoke_untracked.go")))
	assert.NotEqual(t, 0, code, "untracked violation not caught")
	assert.Contains(t, untrackedOut, "ACDSL-GOLANG-EXEC-001")

	s.banner("gate-only rule: gate fires, prompt side silent, verdict logged")
	verdicts := filepath.Join(t.TempDir(), "verdicts.jsonl")
	directive := "//" + `acdsl:ACDSL-SMOKE-900 forbid-addr-lit anchor="^internal/gateonlysmoke/.*\.go$" projected="false" why="smoke: gate-only delivery flag"`
	s.write(smokeGateOnly+"/gateonly.go", "package gateonlysmoke\n\n"+directive+"\n\ntype cfg struct{ n int }\n\nfunc smokeAddr() *cfg { return &cfg{n: 1} }\n")
	planGateOnly, _ := s.run(nil, "project", "-plan", smokeGateOnly+"/other.go")
	assert.NotContains(t, planGateOnly, "ACDSL-SMOKE-900")
	fileGateOnly, _ := s.run(nil, "project", "-file", smokeGateOnly+"/gateonly.go")
	assert.Contains(t, fileGateOnly, "projected "+smokeGateOnly+"/gateonly.go (7 rule(s))")
	assert.NotContains(t, strings.Join(s.lines(smokeGateOnly+"/gateonly.go"), "\n"), "- [ACDSL-SMOKE-900]")
	gateEnv := []string{"ACDSL_VERDICTS_ENABLED=1", "ACDSL_VERDICTS_PATH=" + verdicts}
	checkGateOnly, code := s.run(gateEnv, "check", "-rule", "ACDSL-SMOKE-900")
	assert.NotEqual(t, 0, code, "gate-only violation not caught")
	assert.Contains(t, checkGateOnly, "ACDSL-SMOKE-900")
	logged, err := os.ReadFile(verdicts)
	require.NoError(t, err)
	assert.Contains(t, string(logged), `"id":"ACDSL-SMOKE-900","projected":false`)
	verdictsOut, _ := s.run([]string{"ACDSL_VERDICTS_PATH=" + verdicts}, "verdicts")
	assert.Contains(t, verdictsOut, "ACDSL-SMOKE-900")
	require.NoError(t, os.RemoveAll(s.path(smokeGateOnly)))

	s.banner("task contract lifecycle: drop an .acdsl file anywhere (expect: default gate unaffected)")
	// live task contracts shift the counts; assert relative to the pre-drop baseline
	baseOut, _ := s.run(nil, "check", "-lifetime", "task")
	baseTask := 0
	if anyRulesOK.MatchString(baseOut) {
		baseTask = s.count(anyRulesOK, baseOut)
	}
	s.write(smokeTask, "//"+`acdsl:TASK-XP-001 symbol-exists anchor="^internal/xptask/x\.go$" lifetime="task" symbol="Ping" why="smoke: planned symbol"`+"\n")

	s.banner("task gate red while planned artifact missing (expect: not yet present)")
	taskOut, code := s.run(nil, "check", "-lifetime", "task")
	assert.NotEqual(t, 0, code, "missing planned artifact not flagged")
	assert.Contains(t, taskOut, "planned artifact not yet present")

	s.banner("task gate green once the planned symbol lands (expect: baseline+1 rules OK)")
	s.write("internal/xptask/x.go", "package xptask\n\nfunc Ping() {}\n")
	taskGreen, _ := s.run(nil, "check", "-lifetime", "task")
	assert.Contains(t, taskGreen, fmt.Sprintf("%d rule(s) OK", baseTask+1))

	s.banner("lifetime all gates doctrine + task together (expect: doctrine+baseline+1 rules OK)")
	allOut, _ := s.run(nil, "check", "-lifetime", "all")
	assert.Contains(t, allOut, fmt.Sprintf("%d rule(s) OK", doctrine+baseTask+1))

	s.banner("retire by rename: file no longer ends in .acdsl (expect: baseline task rules)")
	require.NoError(t, os.Rename(s.path(smokeTask), s.path(smokeTask+".off")))
	archivedOut, _ := s.run(nil, "check", "-lifetime", "task")
	assert.Contains(t, archivedOut, fmt.Sprintf("%d rule(s) OK", baseTask))
	require.NoError(t, os.Remove(s.path(smokeTask+".off")))
	require.NoError(t, os.RemoveAll(s.path("internal/xptask")))
}
