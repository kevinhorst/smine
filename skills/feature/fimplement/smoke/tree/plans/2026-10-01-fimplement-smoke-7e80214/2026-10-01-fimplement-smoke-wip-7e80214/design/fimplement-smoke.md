# fimplement smoke — Implementation Plan

target: `opus`, refuted: `yes (no premises)`

- **Standing test plan:** run with `/fimplement fimplement-smoke.md` in a fresh worktree; the worktree is thrown away afterwards and this plan is never archived.

## TLDR
- **Dummy package:** three tiny Go files in a new package nobody uses.
- **Why:** one fixed input that drives a whole fimplement run, so a skill or gate change is checked the same way every time.
- **What it exercises:** plan lookup by file name, frozen rules, two-agent fan-out, phase gates, a rule-covered command, audit and package commit.
- **Result:** a pass or fail per row of the Verification table; the code is never merged.

## Scope
- **In:**
  - **dummy package**
    - `internal/smoke` with three functions and their tests — [D1](#decisions)
  - **frozen rule**
    - one rule, `Bash(lsof *)`, used by one verification row — [D2](#decisions)
  - **run observations**
    - hook lines, fan-out, gates, commit, report — [D3](#decisions)
- **Out:**
  - **merging the result**
    - the run's worktree is discarded — [D1](#decisions)

## Assumptions
N/A — the plan rests on no external premise; it is its own input

## Drivers
| ID | Origin | Observed | Wanted | Impact |
|---|---|---|---|---|
| DR1 | "I am annoyed by this ad-hoc shit with the smoke tests. We need a definition for this … for now fimplement gets a dummy plan which gets implemented with fimplement fimplement-smoke.md" | every check of a fimplement change was improvised from a different throwaway plan | one fixed plan and one fixed list of things the run must show | behavior-preserving |

## Decisions
| ID | Problem [AGENT-ONLY] | Decision [AGENT-ONLY] | Brief | Decision (brief) | Consequences | Why | Rejected | Facts [AGENT-ONLY] |
|---|---|---|---|---|---|---|---|---|
| D1 | The smoke needs a change that is real enough to compile, test and commit, small enough to finish in minutes, and unrelated to any feature so it never collides with other work. It also has to give fimplement a phase with two entries on disjoint files, because that is what triggers the subagent fan-out, and a later one-entry phase that depends on the first. | New package `internal/smoke`: phase 1 has two entries, `Greeting()` and `Farewell()`, each in its own file with its own test; phase 2 has one entry, `Message()`, which joins them. Nothing imports the package. The run happens in a fresh worktree that is removed afterwards; the code is never merged. | **Needs a real change:** the run must build, test and commit something. | **Dummy package:** three functions in `internal/smoke`, two phases. | **Never merged:** every run starts from a tree without the package. | **Controllable:** the same input and the same expected output every run. | **Edit an existing file** — collides with real work<br>**Docs-only change** — no build or test gate to observe | — |
| D2 | A run without a frozen rule only shows that the hook reports "not frozen" and that fimplement continues. The rule path — record lookup, apply, a command that would be denied without it — needs one rule and one command that uses it. | Verification runs `lsof -i :6001`, which the gate denies without the rule `Bash(lsof *)`. The plan dir is frozen once per machine with that rule; the hook applies the record on every run. The command's own exit code is not the check: with nothing listening it exits 1, and that still counts, because it ran. | **Rule path untested:** an empty rule set proves nothing about applying rules. | **One rule, one use:** `lsof` runs only because the frozen rule admits it. | **Freeze per machine:** a machine without the record runs the row as a denied command. | **Debuggable:** one rule traced to one row. | **`kill`** — needs a process to kill<br>**No rule** — skips the path under test | — |
| D3 | A smoke that only checks its own code says nothing about fimplement. What must be observed is how the run went: which hook lines arrived, whether two agents were spawned for phase 1, whether each phase was gated, whether the commit went through package-commit, and what the report says. These are observations of the session, not commands. | The Verification table carries them as `live` rows, each with the exact text or event to look for. The person or session judging the run reads the smoke session with peek and marks each row. | **Run itself unchecked:** green code does not show the skill behaved. | **Observation rows:** the table lists what the session must show. | **Judged from outside:** a second look at the session is part of the smoke. | **Debuggable:** a failing row names the step that went wrong. | **Self-reported pass** — the run would grade itself | — |

## Open questions
N/A — fdesign decides every question

## Changes
| File | Kind | Entry |
|---|---|---|
| `internal/smoke/greeting.go` | new | [Greeting (new)](#greeting-new) |
| `internal/smoke/greeting_test.go` | new | [Greeting (new)](#greeting-new) |
| `internal/smoke/farewell.go` | new | [Farewell (new)](#farewell-new) |
| `internal/smoke/farewell_test.go` | new | [Farewell (new)](#farewell-new) |
| `internal/smoke/message.go` | new | [Message (new)](#message-new) |
| `internal/smoke/message_test.go` | new | [Message (new)](#message-new) |

### Greeting (new)
location: `internal/smoke/greeting.go`, `internal/smoke/greeting_test.go`
phase: 1

```go
// Package smoke is the dummy change of the fimplement smoke plan; nothing
// imports it.
package smoke

// Greeting is the opening word of the smoke message.
func Greeting() string {
	return "hello"
}
```

```go
package smoke

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGreeting(t *testing.T) {
	assert.Equal(t, "hello", Greeting())
}
```

### Farewell (new)
location: `internal/smoke/farewell.go`, `internal/smoke/farewell_test.go`
phase: 1

```go
package smoke

// Farewell is the closing word of the smoke message.
func Farewell() string {
	return "goodbye"
}
```

```go
package smoke

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFarewell(t *testing.T) {
	assert.Equal(t, "goodbye", Farewell())
}
```

### Message (new)
location: `internal/smoke/message.go`, `internal/smoke/message_test.go`
phase: 2

```go
package smoke

// Message joins the greeting and the farewell.
func Message() string {
	return Greeting() + ", " + Farewell()
}
```

```go
package smoke

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage(t *testing.T) {
	assert.Equal(t, "hello, goodbye", Message())
}
```

## Contracts
N/A — the package has no caller and crosses no boundary

## Tests
| Location.Method | Cases | Comment |
|---|---|---|
| `greeting_test.go` `TestGreeting` | returns `hello` | direct test, one scenario |
| `farewell_test.go` `TestFarewell` | returns `goodbye` | direct test, one scenario |
| `message_test.go` `TestMessage` | returns `hello, goodbye` | direct test, one scenario |

## Verification
| ID | Check | Command | Expected | Surface |
|---|---|---|---|---|
| V1 | package builds | `go build ./internal/smoke/...` | exit 0 | local |
| V2 | package tests | `go test ./internal/smoke/...` | `ok` | local |
| V3 | formatting | `gofmt -l internal/smoke` | no output | local |
| V4 | rule-covered command runs | `lsof -i :6001` | the command runs and is not denied by the gate; its exit code does not matter | live |
| V5 | full gate | `make audit` | exit 0 | local |
| V6 | commit landed | `git log --oneline -3` | one commit `Import initial version of package internal.smoke` | local |
| V7 | plan found by file name | the session's first Read after `/fimplement fimplement-smoke.md` | it reads `plans/2026-10-01-fimplement-smoke-wip-7e80214/design/fimplement-smoke.md` without searching for it, and the report quotes the hook's `- plan:` line | live |
| V8 | frozen rule applied | the session's `lsof -i :6001` tool call | the call is not an error, and the report quotes the hook's `- Permissions:` line naming `Bash(lsof *)` | live |
| V9 | phase 1 fanned out | the session's subagent list | two subagents on model `opus`, one per phase 1 entry; none for phase 2 | live |
| V10 | each phase gated | the session's tool calls | a build and a test call after phase 1 and again after phase 2 | live |
| V11 | commit through the skill | the session's skill invocations | `package-commit` invoked after the green audit | live |
| V12 | report against the plan | the session's last message | every row V1 to V6 named with its result; no permission gap reported | live |
| V13 | degenerate: machine without the record | the hook lines on a machine where the plan dir was never frozen | `- Permissions: none applied — … is not frozen`; the run continues and reports `Bash(lsof *)` as the candidate rule for V4 | live |

- **V7 to V13** are read from the smoke session with `/peek <session title>`; the smoke session does not grade them itself.

## Current state [AGENT-ONLY]
N/A — new route

## Target state [AGENT-ONLY]
N/A — new route

## Baseline (verified) [AGENT-ONLY]
- **Base branch:** `claude/permission-jailbreak-fix-cont-a72b54` at `7e80214`

| ID | Fact | Needed for | Location |
|---|---|---|---|
| F1! | `internal/smoke` does not exist | [D1](#decisions) | `ls internal`, 2026-10-01 |
| F2! | the gate denies `lsof -i :6001` with candidate `Bash(lsof *)` and admits every other command of this plan | [D2](#decisions) | `toolpolicy check` run 2026-10-01 |
| F3 | a phase with two entries on disjoint files is what makes fimplement spawn one subagent per entry | [D1](#decisions) | [fimplement.cue](skills/feature/fimplement/fimplement.cue), entry "Phase fan-out" |

## Exemplar & reuse [AGENT-ONLY]
| Existing | Used for |
|---|---|
| `testify/assert` | the three direct tests |

- **No exemplar:** none needed — three one-line functions.

## Test runbook [AGENT-ONLY]
N/A — no callable surface

## Addenda
N/A — no change since creation

## Changelog
| Date | Trigger | What changed |
|---|---|---|
| — | initial | plan created |
| 2026-10-01 | local: first smoke run | V7 and V8 judged from tool calls and the report: peek does not show hook lines, so the original rows could not be checked from outside |
| 2026-10-02 | skill-test-layers D10 | moved into fimplement's smoke fixture under the same plan dir name; `make skill-smoke SKILL=fimplement` runs it and judges the terminal result plus `go test` over the six files; V7 to V13 stay observations read from the session |
