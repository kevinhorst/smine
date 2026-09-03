# Language + Style setting (non-developer install) — Implementation Plan

## TLDR

- One per-install **presentation profile** file (`~/.claude/context/global/presentation-profile.md`): language + audience header, German-directive body.
- Every Claude session on the machine inherits the directive via the existing global-context hook, extended to scan a machine-global directory.
- The nightly consolidate stage threads `language de` into `/smine-consolidate proposals` — the drift-correction gate over the proposal store.
- The config server reads the same file: German UI overlay via a template `t` func, `lang` attribute from the profile, reduced navigation (Übersicht/Sitzungen/Vorschläge only), proposal cards without gate/tags/target internals, snippets relabeled "Technisches Detail".
- Kevin's machine (no profile file) behaves exactly as today: English, full UI, no consolidate arg.
- Provisioning: a German profile template ships in the repo; `SMINE_PRESENTATION_PROFILE=de bash install.sh` copies it into place.

## Context

- Approved concept: [plans/language_style_setting/concept/concept.md](plans/language_style_setting/concept/concept.md) — all decisions closed, binding.
- Problem: smine is English/developer-facing at every layer; a German non-developer install needs German, semi-casual, jargon-free presentation everywhere the user reads.
- Cause anchors: hardcoded English UI (internal/server/templates/layout.html:3, :26-40), consolidate invoked without language (routines/smine-nightly/run.sh:147), English-only mandate (settings/claude_code/CLAUDE.md:16).
- Constraint: the profile is per-install state, never repo content; operator surfaces (logs, results.jsonl, repo docs) stay English.
- Constraint: model drift (Opus/Fable) is the known failure mode — prevention (directive in every session) plus correction (consolidate language pass) are both required.

## Drivers

N/A — new route.

## Scope

- **In:**
  - **profile-file:** parseable per-install profile + German template in repo
  - **hook:** machine-global scan in global-context.sh
  - **nightly:** language threading into the consolidate stage
  - **server:** profile load, `t`/`langAttr`/`isDeveloperAudience` FuncMap, German catalog
  - **reduced-surface:** nav allowlist, overview tile filter, proposal-card internals hidden, snippets relabeled
  - **install (macOS):** `SMINE_PRESENTATION_PROFILE` provisioning knob in install.sh
  - **install (Windows):** installer wizard page with a hard-coded profile selection, threaded through `configserver -install`
  - **claude-md:** supersede clause on the English-only rule
- **Out:**
  - **config-server edit page** for the profile (backlog per concept)
  - **full two-locale catalog** across Skills/Context/Repos/Tools/Checklist pages (backlog)
  - **welcome/tutorial rewrite** for non-developers — Welcome is hidden for the audience instead
  - **console/install output localization**
  - **hard pre-render style verifier** (consolidate-as-gate only, per concept)
- **Not changed:**
  - **smine-consolidate SKILL.md** — the `language <lang>` contract already exists (SKILL.md:32)
  - **proposal schema / stores** — prose language is content, not schema
  - **per-repo context packs** — sync_context.sh global copying stays as is
- **Deferred findings:**
  - **overview tile Detail strings** (`fmt.Sprintf` composites like "3/12 done") stay English-ish; only labels translate in MVP
  - **on a non-developer machine the synced CLAUDE.md is still Kevin's developer doctrine** — a non-developer settings variant is a separate feature

## Assumptions

| Assumption | Reality | Location |
|---|---|---|
| Concept: profile "materialized into context/global/" reaches every session | The hook reads `$DOCS_DIR/global/*.md` **cwd-relative per repo**, not machine-global — a hook extension is required, the concept's mechanism alone is insufficient | cmd/hooks/global-context.sh:16,24 |
| Concept: consolidate has an unused `language` arg | Confirmed — contract at skills/smine/smine-consolidate/SKILL.md:32, unthreaded at routines/smine-nightly/run.sh:147 | verified |
| Concept: no i18n layer exists in the server | Confirmed — literal English in templates and Go; no message catalog, `lang="en"` hardcoded | internal/server/templates/layout.html:3 |

## Current state

N/A — new route.

## Target state

N/A — new route.

## Behavior contract

N/A — new route. (Key invariant carried in D2/Verification: absent profile ⇒ byte-identical behavior to today.)

## Decisions

| ID | Problem | Facts | Decision | Why |
|---|---|---|---|---|
| <a id="d1"></a>D1 | Profile format + location | F1!, F13 | One markdown file at `~/.claude/context/global/presentation-profile.md`: `---`-delimited `language:`/`audience:` header, directive body. Parsed by Go and by `sed` in run.sh; body injected verbatim into sessions. [USER: per-install file, per concept] | Single source of truth for three consumers (hook, server, nightly); debuggable — the user-readable file IS the state |
| <a id="d2"></a>D2 | Missing/malformed profile | F1! | Fail open to defaults (`en`, `developer`): absent file, unreadable file, or missing keys all yield today's behavior; unknown `audience` values count as developer | Kevin's machines must be untouched without opting in; degrades predictably (reliable axis) |
| <a id="d3"></a>D3 | How the directive reaches every session | F1! | Extend global-context.sh to scan `$HOME/.claude/context/global/*.md` before the repo-local `$DOCS_DIR/global` scan | Controllable via the existing kill switch; no new hook registration; per-repo mechanism untouched |
| <a id="d4"></a>D4 | Server plumbing: per-request struct vs per-install closures | F4!, F11 | Per-install FuncMap closures (`t`, `langAttr`, `isDeveloperAudience`) over the profile loaded once in `server.New`; no page-struct changes | Mirrors the existing `initWelcome`/`appVersion` closure pattern; touching ~12 independent page structs is churn with no gain for a per-install value |
| <a id="d5"></a>D5 | Catalog shape | F3! | `internal/server/i18n.go`: `map[string]string` keyed by the English source string, identity fallback on miss | Keys double as the English default — no key indirection to invent; a missing entry degrades to English, never breaks a page |
| <a id="d6"></a>D6 | Reduced surface mechanics | F6!, F7! | Nav: gate dev-only anchors on `isDeveloperAudience` (incl. Welcome + Peek). Overview: non-dev branch appends only `sessionTiles` + `proposalsTile`. Card: hide target/gate/tags badges + gate detail row; snippet summary becomes "Technisches Detail". Direct URLs to hidden pages stay reachable | Render-time filter on the same data — the operator flips the profile back and gets the full UI; no forked handlers |
| <a id="d7"></a>D7 | Nightly threading | F2! | run.sh parses `language:` from the profile with `sed`; when set and ≠ `en`, the consolidate prompt becomes `/smine-consolidate proposals language <lang>` | Uses the skill's existing contract; consolidate is the correction gate per concept decision |
| <a id="d8"></a>D8 | Provisioning | F13 | Template `settings/claude_code/presentation-profile.de.md` in repo; `install.sh` copies it to the live path only when `SMINE_PRESENTATION_PROFILE=de` is set [USER: written by install/sync, per concept] | Explicit opt-in — a default install never plants a profile; repo carries content, not state |
| <a id="d9"></a>D9 | English-only rule conflict | F8 | Append a supersede sentence to CLAUDE.md's artifact-language rule: an installed presentation profile wins for end-user-visible artifacts | Without it, every session holds two contradictory instructions and drift wins |
| <a id="d10"></a>D10 | Profile struct/API shape | F5 | `presentationProfile` struct + `loadPresentationProfile(path)` in `internal/server/presentation.go` (package server, unexported); read-only — no store, no persistence API | The server only reads; cloning the write-through `disabledHookStore` pattern would be speculative persistence |
| <a id="d11"></a>D11 | Windows install-time selection mechanics | F14!, F15! | A `TInputOptionWizardPage` radio group in smine.iss (`Default — English, developer` checked; `Deutsch — nicht-technisch`), hard-coded list; a `[Code]` `ProfileArg` function appends ` -presentation-profile=de` to the existing `[Run]` `-install` invocation; `runInstall` (install_windows.go) copies the repo template into `%USERPROFILE%\.claude\context\global\presentation-profile.md` [USER: installer exposes hard-coded profiles] | Follows the only two existing installer patterns (custom wizard page, flag threading à la `-init-welcome`); the copy lives in the Go installer that already owns Windows home-dir deployment — no new script surface |

## Open questions

_None — empty at approval._

## Baseline (verified)

Base branch: `claude/vigorous-mcnulty-f43ad0` (session worktree, clean at start).

| ID | Fact | Needed for | Location |
|---|---|---|---|
| <a id="f1"></a>F1! | global-context.sh injects `$DOCS_DIR/global/*.md`, `DOCS_DIR` = `AGENT_CONTEXT_DIR_DEFAULT` or `docs`, cwd-relative; kill switch `GLOBAL_CONTEXT_ENABLED` | [D1](#d1), [D2](#d2), [D3](#d3) | cmd/hooks/global-context.sh:15-24 |
| <a id="f2"></a>F2! | Nightly consolidate stage runs `claude -p "/smine-consolidate proposals"` with no language; the skill defines `language <lang>` ("write reworded prose in <lang>") | [D7](#d7) | routines/smine-nightly/run.sh:147, skills/smine/smine-consolidate/SKILL.md:32 |
| <a id="f3"></a>F3! | No i18n layer: nav is hardcoded anchors, `<html lang="en">` literal, page/card text literal English in templates and Go | [D5](#d5), §layout, §card | internal/server/templates/layout.html:3,26-40 |
| <a id="f4"></a>F4! | FuncMap closures over `opts` already gate nav and feed the brand (`initWelcome`, `appVersion`, `peekDashboardURL`); FuncMap built inline in `New`, `.Funcs(funcs).ParseFS` at server.go:182 | [D4](#d4) | internal/server/server.go:96-182 |
| <a id="f5"></a>F5 | The only preference-persistence precedent is the write-through `disabledHookStore` sidecar; no app-profile store exists | [D10](#d10) | internal/server/disabled_hooks.go:33-139 |
| <a id="f6"></a>F6! | `handleOverview` builds tiles by appending per-domain builders: welcome, sessionTiles, settingsTiles, skillTiles, repoTile, contextTile, proposalsTile, routineTile, toolsTile, checklistTile | [D6](#d6), §overview | internal/server/overview.go:91-104 |
| <a id="f7"></a>F7! | Card internals sit on known lines: target badge :6, gate band :7-10, tags :11, gate detail :59-64, snippets (already a collapsed `<details>`) :77-82; card is `{{define "_proposal_card.html"}}` | [D6](#d6), §card | internal/server/templates/_proposal_card.html |
| <a id="f8"></a>F8 | English-only mandate: "Handoff/doc artifacts: always English …" | [D9](#d9) | settings/claude_code/CLAUDE.md:16 |
| <a id="f9"></a>F9 | Handler tests use `newTestServer(t, opts)` (temp settings.json + `New`) and assert rendered HTML via `httptest` (`TestNavPeekLink`) | §tests | internal/server/server_test.go:14-39 |
| <a id="f10"></a>F10 | main.go resolves per-install paths via flags with home-derived defaults (`-settings` → `config.DefaultPath()`) | §main | cmd/configserver/main.go:43 |
| <a id="f11"></a>F11 | Templates execute by file name via `renderFragment` → `ExecuteTemplate`; a parse error in any template fails `New` | §templates | internal/server/render.go:19-21, server.go:182 |
| <a id="f12"></a>F12 | ACDSL projections: Go files — gofmt, ctx-first signatures, no swallowing enum default, shell.Run for children (no children added here); hook scripts — shellcheck + bash 3.2 (no `declare -A`/`mapfile`) | §changes, §hook | `go run ./cmd/acdsl project -plan`, run this session |
| <a id="f13"></a>F13 | sync_context.sh copies **repo** `context/global/*.md` into per-repo packs — repo-level global docs are repo content, so a per-install profile must not live there | [D1](#d1), [D8](#d8) | cmd/sync/sync_context.sh:217-221 |
| <a id="f14"></a>F14! | smine.iss has no `[Tasks]`/`[Components]`; the only interactive selection is the `RepoPage` dir page (`CreateInputDirPage`, `InitializeWizard`); `[Run]` invokes `{code:RepoBin}\configserver.exe` with `Parameters: "-install"` only; `/DAppVersion=` is the sole define | [D11](#d11), §installer | installer/windows/smine.iss:38-43,55-63 |
| <a id="f15"></a>F15! | Windows install flow: install.ps1 `-InitWelcome` switch → `configserver -install -init-welcome=…` → `runInstall(ctx, …)` in install_windows.go, which locates Git Bash and runs the same three sync scripts as install.sh; `main.go:56-57,112-114` parses/dispatches | [D11](#d11), §installer | install.ps1:61-64, cmd/configserver/install_windows.go:79-91,341-349 |
| <a id="f16"></a>F16 | Installer validation is `make installer-check` (dockerized `amake/innosetup` iscc against a dummy dist/) — a tag/CI run must never be the first iscc compile; CI builds the real setup exe tag-gated | §verification | Makefile:128-141, .github/workflows/ci.yml:63-88 |
| <a id="f17"></a>F17 | On Windows, hooks and sync scripts run under Git Bash (`shell.BashPath()`, `SMINE_BASH`); sync_settings.sh branches on MINGW/MSYS with `cygpath -m`, so `$HOME/.claude/context/global` resolves to `%USERPROFILE%\.claude\context\global` — the hook change in §2 works unmodified on Windows | [D3](#d3) | cmd/sync/sync_settings.sh:12-19, install_windows.go:88 |

## Exemplar & reuse

| Existing | Used for |
|---|---|
| FuncMap closure pattern (`initWelcome`, server.go:148) | `t` / `langAttr` / `isDeveloperAudience` funcs |
| `newTestServer` + HTML assertions (server_test.go:14) | all new handler tests |
| `config.DefaultPath()` home-resolution (internal/config/config.go:192) | `defaultPresentationPath()` |
| smine-consolidate `language` contract (SKILL.md:32) | nightly threading, no skill change |
| `global-context.env` sourcing + kill switch (global-context.sh:15-21) | machine-global scan rides the same toggle |

- Without exemplar: the German catalog (`i18n.go`) — no prior i18n anywhere in the repo; risk contained by identity fallback (D5).

## Changes

### 1. Presentation profile template (new)

location: `settings/claude_code/presentation-profile.de.md`

```markdown
---
language: de
audience: non-developer
---
# Presentation profile (injected into every session on this machine)

This machine belongs to a German-speaking non-developer. For everything this
user reads — proposal titles, change lines, detail fields, evidence notes,
reports, chat responses — the following overrides any general rule that
artifacts are written in English (that rule still governs code, logs, commit
messages, and repo documents):

- Write all user-visible prose in German.
- Register: semi-casual, semi-professional ("du" is fine, no slang, no
  bureaucratic German).
- Never expose engine internals or developer jargon. Banned terms and their
  plain-German replacements:
  - worktree, branch, commit → "Arbeitskopie" / omit entirely
  - transcript mining, batch → "Auswertung deiner Sitzungen"
  - proposal gate, band, verifier → omit (internal)
  - skill, hook, context pack → "Funktion" / "Automatik"
  - consolidate/dedup → "aufräumen"
- Never translate or alter: code blocks, identifiers, ids, dates, tags, file
  paths, and anything inside proposal `snippets`.
- When a technical fragment must appear, collapse it and label it
  "Technisches Detail".
```

### 2. Machine-global context injection (modified)

location: `cmd/hooks/global-context.sh`
governing gates: ACDSL-SHELL-001 (shellcheck), ACDSL-SHELL-002 (bash 3.2)

```diff
 found=0
-for f in "$DOCS_DIR"/global/*.md; do
-  [ -f "$f" ] || continue
-  if [ "$found" = 0 ]; then
-    echo "===== GLOBAL CONTEXT (injected by hook) ====="
-    echo ""
-    found=1
-  fi
-  echo "===== $f ====="
-  cat "$f"
-  echo ""
-done
+# Machine-global content ($HOME/.claude/context/global) first — the
+# presentation profile lives there — then the repo-local docs dir.
+for dir in "$HOME/.claude/context/global" "$DOCS_DIR/global"; do
+  for f in "$dir"/*.md; do
+    [ -f "$f" ] || continue
+    if [ "$found" = 0 ]; then
+      echo "===== GLOBAL CONTEXT (injected by hook) ====="
+      echo ""
+      found=1
+    fi
+    echo "===== $f ====="
+    cat "$f"
+    echo ""
+  done
+done
```

### 3. Profile loader (new)

location: `internal/server/presentation.go`
gates: gofmt, no-swallowing-enum-default (audience check is an explicit non-dev test per D2, not an enum switch)

```go
package server

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	audienceNonDeveloper = "non-developer"
	languageEnglish      = "en"
)

type presentationProfile struct {
	Audience string
	Language string
}

// loadPresentationProfile reads the per-install profile; any absence or
// parse problem falls back to defaults (English, developer) so machines
// without a profile behave exactly as before (D2).
func loadPresentationProfile(path string) *presentationProfile {
	profile := &presentationProfile{
		Audience: "",
		Language: languageEnglish,
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return profile
	}

	inFrontMatter := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontMatter {
				break
			}
			inFrontMatter = true
			continue
		}
		if !inFrontMatter {
			continue
		}
		if value, ok := strings.CutPrefix(trimmed, "language:"); ok {
			profile.Language = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(trimmed, "audience:"); ok {
			profile.Audience = strings.TrimSpace(value)
		}
	}
	if profile.Language == "" {
		profile.Language = languageEnglish
	}
	return profile
}

func (p *presentationProfile) isDeveloperAudience() bool {
	return p.Audience != audienceNonDeveloper
}

func defaultPresentationPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "context", "global", "presentation-profile.md")
}
```

### 4. German catalog (new)

location: `internal/server/i18n.go`

```go
package server

const languageGerman = "de"

// germanCatalog maps English source strings (the keys double as the English
// default) to their German overlay. A missing key renders the English
// original — the page never breaks (D5).
var germanCatalog = map[string]string{
	"Overview":  "Übersicht",
	"Sessions":  "Sitzungen",
	"Proposals": "Vorschläge",
	"batches":   "Auswertungen",
	"comment (rejection reason or agent instruction)": "Kommentar (Grund oder Hinweis)",
	"details":              "Details",
	"evidence":             "Belege",
	"fields":               "Angaben",
	"postpone":             "später",
	"technical detail":     "Technisches Detail",
	"Reload from disk":     "Neu laden",
	"Sessions analyzed":    "Ausgewertete Sitzungen",
	"Last analysis":        "Letzte Auswertung",
	"Frustration / Positive": "Frust / Positiv",
}

func translate(language, text string) string {
	if language != languageGerman {
		return text
	}
	if translated, ok := germanCatalog[text]; ok {
		return translated
	}
	return text
}
```

- **catalog completion:** the entries above are the grounded set; fimplement sweeps the reduced surface's templates (layout, overview, proposals, sessions_index, sessions_batch, _batch_list, _proposal_card) and `sessionTiles`/`proposalsTile` label strings, adding every literal the non-developer sees. Same file, same map — no structural change.

### 5. Options + boot wiring (modified)

location: `cmd/configserver/main.go`, `internal/server/server.go`

```diff
 	settingsPath := flag.String("settings", config.DefaultPath(), "path to settings.json")
+	presentationPath := flag.String(
+		"presentation",
+		server.DefaultPresentationPath(),
+		"path to the per-install presentation profile (missing file = English/developer defaults)",
+	)
```

- `defaultPresentationPath` is exported as `DefaultPresentationPath` (main.go needs it, mirroring `config.DefaultPath()` — F10); the flag value lands in `Options.PresentationPath`.

```diff
 type Options struct {
 	// ... existing fields (alphabetical) ...
+	PresentationPath string
```

```diff
 func New(opts *Options) (*Server, error) {
+	profile := loadPresentationProfile(opts.PresentationPath)
 	funcs := template.FuncMap{
 		// ... existing funcs ...
+		// t overlays the profile language onto English source strings;
+		// identity when no catalog entry or language is English (D5).
+		"t": func(text string) string {
+			return translate(profile.Language, text)
+		},
+		"langAttr": func() string {
+			return profile.Language
+		},
+		// isDeveloperAudience gates internals-exposing nav and card parts (D6).
+		"isDeveloperAudience": profile.isDeveloperAudience,
 	}
```

- `Server` needs the profile for `handleOverview`: add unexported field `profile *presentationProfile`, set in `New`.

### 6. Reduced navigation + lang attribute (modified)

location: `internal/server/templates/layout.html`

```diff
-<html lang="en">
+<html lang="{{langAttr}}">
```

```diff
 <nav>
-  <a href="/" {{if eq .Page "overview"}}class="active"{{end}}>Overview</a>
-  {{if initWelcome}}<a href="/welcome" {{if eq .Page "welcome"}}class="active"{{end}}>Welcome</a>{{end}}
-  <a href="/config/claude" {{if or (eq .Page "config-claude") (eq .Page "config-codex")}}class="active"{{end}}>Config</a>
-  <a href="/sessions" {{if eq .Page "sessions"}}class="active"{{end}}>Sessions</a>
-  <a href="/proposals" {{if eq .Page "proposals"}}class="active"{{end}}>Proposals</a>
-  <a href="/scripts/skills" {{if eq .Page "skills"}}class="active"{{end}}>Skills</a>
-  <a href="/context" {{if eq .Page "context"}}class="active"{{end}}>Context</a>
-  <a href="/repos" {{if eq .Page "repos"}}class="active"{{end}}>Repos</a>
-  <a href="/routines" {{if eq .Page "routines"}}class="active"{{end}}>Routines</a>
-  <a href="/tools" {{if eq .Page "tools"}}class="active"{{end}}>Tools</a>
-  <a href="/docs/checklist" {{if eq .Page "checklist"}}class="active"{{end}}>Checklist</a>
-  {{if peekDashboardURL}}<a href="{{peekDashboardURL}}">Peek</a>{{end}}
+  <a href="/" {{if eq .Page "overview"}}class="active"{{end}}>{{t "Overview"}}</a>
+  {{if and initWelcome isDeveloperAudience}}<a href="/welcome" {{if eq .Page "welcome"}}class="active"{{end}}>Welcome</a>{{end}}
+  {{if isDeveloperAudience}}<a href="/config/claude" {{if or (eq .Page "config-claude") (eq .Page "config-codex")}}class="active"{{end}}>Config</a>{{end}}
+  <a href="/sessions" {{if eq .Page "sessions"}}class="active"{{end}}>{{t "Sessions"}}</a>
+  <a href="/proposals" {{if eq .Page "proposals"}}class="active"{{end}}>{{t "Proposals"}}</a>
+  {{if isDeveloperAudience}}
+  <a href="/scripts/skills" {{if eq .Page "skills"}}class="active"{{end}}>Skills</a>
+  <a href="/context" {{if eq .Page "context"}}class="active"{{end}}>Context</a>
+  <a href="/repos" {{if eq .Page "repos"}}class="active"{{end}}>Repos</a>
+  <a href="/routines" {{if eq .Page "routines"}}class="active"{{end}}>Routines</a>
+  <a href="/tools" {{if eq .Page "tools"}}class="active"{{end}}>Tools</a>
+  <a href="/docs/checklist" {{if eq .Page "checklist"}}class="active"{{end}}>Checklist</a>
+  {{if peekDashboardURL}}<a href="{{peekDashboardURL}}">Peek</a>{{end}}
+  {{end}}
   <span class="nav-brand">smine.<span class="nav-version">v{{appVersion}}</span></span>
```

### 7. Overview tile filter (modified)

location: `internal/server/overview.go`

```diff
 func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
 	var tiles []overviewTile
+	// Non-developer installs see only the session and proposal tiles —
+	// the remaining tiles ARE engine internals (D6).
+	if !s.profile.isDeveloperAudience() {
+		tiles = append(tiles, s.sessionTiles(parseWindow(r))...)
+		tiles = append(tiles, s.proposalsTile())
+		data := overviewPage{Page: pageOverview, Tiles: tiles, Title: translate(s.profile.Language, "Overview")}
+		s.renderFragment(w, tmplOverview, data)
+		return
+	}
 	if welcome, show := s.welcomeTile(r.Context()); show {
 		// ... existing body unchanged ...
```

- Tile `Label` rendering switches to the overlay in the template: `{{.Label}}` → `{{t .Label}}` in `overview.html` (identity for untranslated labels).

### 8. Proposal card for non-developers (modified)

location: `internal/server/templates/_proposal_card.html`

```diff
     <span class="label"><strong>{{.Proposal.Title}}</strong></span>
-    {{if .Proposal.Target}}<span class="badge badge-dim">→ {{.Proposal.Target}}</span>{{end}}
-    {{if .Proposal.Gate.Band}}
+    {{if isDeveloperAudience}}
+    {{if .Proposal.Target}}<span class="badge badge-dim">→ {{.Proposal.Target}}</span>{{end}}
+    {{if .Proposal.Gate.Band}}
       {{if eq .Proposal.Gate.Band "J"}}<span class="badge badge-action">band J — prose</span>
       {{else}}<span class="badge badge-dim">band {{.Proposal.Gate.Band}}</span>{{end}}
-    {{end}}
-    {{range .Proposal.Tags}}<span class="badge badge-dim">{{.}}</span>{{end}}
+    {{end}}
+    {{range .Proposal.Tags}}<span class="badge badge-dim">{{.}}</span>{{end}}
+    {{end}}
```

```diff
-    <input type="text" name="comment" maxlength="500"
-           placeholder="comment (rejection reason or agent instruction)" value="{{.PendingComment}}">
+    <input type="text" name="comment" maxlength="500"
+           placeholder="{{t "comment (rejection reason or agent instruction)"}}" value="{{.PendingComment}}">
     <button type="submit" name="vote" value="+">+</button>
     <button type="submit" name="vote" value="-">−</button>
-    <button type="submit" name="vote" value="p">postpone</button>
+    <button type="submit" name="vote" value="p">{{t "postpone"}}</button>
```

```diff
   <details class="card-more">
-    <summary>details{{with .Proposal.Fields}} · {{len .}} fields{{end}}{{with .Evidence}} · {{len .}} evidence{{end}}</summary>
+    <summary>{{t "details"}}{{with .Proposal.Fields}} · {{len .}} {{t "fields"}}{{end}}{{with .Evidence}} · {{len .}} {{t "evidence"}}{{end}}</summary>
     {{range .Proposal.Fields}}
     ...
     {{end}}
-    {{if .Proposal.Gate.Band}}
+    {{if and isDeveloperAudience .Proposal.Gate.Band}}
     <div class="finding">
       <span class="badge badge-dim">gate</span>
       ...
```

```diff
           {{range .Item.Snippets}}
           <details class="snippet">
-            <summary>{{.Kind}}{{if .Lang}} · {{.Lang}}{{end}}{{if .Source}} · from {{.Source}}{{end}}</summary>
+            <summary>{{if isDeveloperAudience}}{{.Kind}}{{if .Lang}} · {{.Lang}}{{end}}{{if .Source}} · from {{.Source}}{{end}}{{else}}{{t "technical detail"}}{{end}}</summary>
             <pre><code>{{.Code}}</code></pre>
```

- Same disabled-form placeholder (line 34) and the static texts in `proposals.html`, `sessions_index.html`, `sessions_batch.html`, `_batch_list.html` get the identical `{{t "…"}}` wrapping — pattern above, applied per literal during the catalog sweep (§4). The card's `details`/`_proposal_card.html` diffs above are the exemplar.

### 9. Nightly language threading (modified)

location: `routines/smine-nightly/run.sh`
gates: ACDSL-SHELL-002 (bash 3.2 — plain `sed`, no arrays needed)

```diff
 # ---- Stage 1.5: consolidate proposals (dedup, re-home, schema/audit gate) ----
 consolidate_status=0
+# Presentation profile: thread the install's language into the consolidate
+# rewording pass (the style-correction gate for the proposal store).
+consolidate_prompt="/smine-consolidate proposals"
+presentation_profile="$HOME/.claude/context/global/presentation-profile.md"
+if [[ -f "$presentation_profile" ]]; then
+  profile_language=$(sed -n 's/^language:[[:space:]]*//p' "$presentation_profile" | head -1)
+  if [[ -n "$profile_language" && "$profile_language" != "en" ]]; then
+    consolidate_prompt="/smine-consolidate proposals language $profile_language"
+  fi
+fi
 consolidate_tools="$(routine_allowed_tools smine-consolidate)"
 ...
-consolidate_output=$(routine_run_claude 3600 claude -p "/smine-consolidate proposals" \
+consolidate_output=$(routine_run_claude 3600 claude -p "$consolidate_prompt" \
```

### 10. Install provisioning, macOS (modified)

location: `install.sh` (run from the main checkout — never a worktree)

```diff
   echo "-> Syncing settings ..."
   ...
+if [[ -n "${SMINE_PRESENTATION_PROFILE:-}" ]]; then
+  echo "-> Installing presentation profile (${SMINE_PRESENTATION_PROFILE}) ..."
+  mkdir -p "$HOME/.claude/context/global"
+  cp "$REPO_DIR/settings/claude_code/presentation-profile.${SMINE_PRESENTATION_PROFILE}.md" \
+     "$HOME/.claude/context/global/presentation-profile.md"
+fi
```

### 11. Install provisioning, Windows flag (modified)

location: `cmd/configserver/main.go`, `cmd/configserver/install_windows.go`
mirrors: the `-init-welcome` threading (F15)

```diff
 	initWelcome := flag.Bool("init-welcome", false, ...)
+	presentationProfileId := flag.String(
+		"presentation-profile",
+		"",
+		"presentation profile template id to install (e.g. de); empty = none",
+	)
```

```diff
-	os.Exit(runInstall(ctx, *addr, *initWelcome, *peekPort, *peekControlPort))
+	os.Exit(runInstall(ctx, *addr, *initWelcome, *peekPort, *peekControlPort, *presentationProfileId))
```

- `runInstall` gains the parameter (non-Windows stub updated identically); after `runSyncs` succeeds:

```go
// installPresentationProfile copies the selected repo template to the
// per-install profile path; an empty id installs nothing (D11).
func installPresentationProfile(home, profileId, repoDir string) error {
	if profileId == "" {
		return nil
	}
	source := filepath.Join(repoDir, "settings", "claude_code", "presentation-profile."+profileId+".md")
	targetDir := filepath.Join(home, ".claude", "context", "global")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return errors.Wrap(err, "installPresentationProfile: Failed to create target dir")
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return errors.Wrapf(err, "installPresentationProfile: Failed to read template %s", profileId)
	}
	target := filepath.Join(targetDir, "presentation-profile.md")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return errors.Wrap(err, "installPresentationProfile: Failed to write profile")
	}
	return nil
}
```

- Error/import shapes follow the file's existing error package; if install_windows.go uses stdlib `fmt.Errorf`, mirror that instead of `pkg/errors` (nearest-pattern wins).

### 12. Installer wizard selection (modified)

location: `installer/windows/smine.iss`
mirrors: `RepoPage` (F14)

```diff
 var
   RepoPage: TInputDirWizardPage;
+  ProfilePage: TInputOptionWizardPage;
```

```diff
 procedure InitializeWizard;
 begin
   ... existing RepoPage setup ...
+  ProfilePage := CreateInputOptionPage(RepoPage.ID,
+    'Presentation profile', 'Who is this machine set up for?',
+    'Select how smine presents itself on this machine. This can be changed later by replacing the profile file.',
+    True, False);
+  ProfilePage.Add('Default — English, developer');
+  ProfilePage.Add('Deutsch — nicht-technisch (German, non-developer)');
+  ProfilePage.Values[0] := True;
 end;
+
+function ProfileArg(Param: string): string;
+begin
+  Result := '';
+  if ProfilePage.Values[1] then
+    Result := ' -presentation-profile=de';
+end;
```

```diff
 [Run]
-Filename: "{code:RepoBin}\configserver.exe"; Parameters: "-install"; ...
+Filename: "{code:RepoBin}\configserver.exe"; Parameters: "-install{code:ProfileArg}"; ...
```

- The option list is hard-coded by design [USER]; adding a profile later = one template file + one `ProfilePage.Add` line + one mapping branch.
- `install.ps1` gains a matching `-PresentationProfile ""` parameter forwarded as `-presentation-profile=` (same shape as `-InitWelcome`, install.ps1:61-64) so scripted installs can select without the wizard.

### 13. English-only rule supersede (modified)

location: `settings/claude_code/CLAUDE.md` (line 16)

```diff
-- Handoff/doc artifacts: always English regardless of the working language; expand domain abbreviations ...
+- Handoff/doc artifacts: always English regardless of the working language — unless an installed presentation profile (~/.claude/context/global/presentation-profile.md) overrides this for end-user-visible artifacts; code, logs, commits, and repo docs stay English regardless. Expand domain abbreviations ...
```

## Hot items

- **UI change (ACTION-CONCEPT-HOT-007):** rendered mockup of the non-developer German proposals page presented in chat alongside this plan (reduced nav Übersicht/Sitzungen/Vorschläge, card without target/band/tags badges, German vote controls, collapsed "Technisches Detail", footer noting exactly which of today's elements are hidden). The template diffs in §6/§8 are the implementation of that picture.
- No SQL/CTEs, no concurrency, no new interfaces or generics, no generated formats, no anonymous structs. No validation/guard logic is weakened — the profile parser only adds a read path with fail-open defaults (D2).

## Tests

| Location.Method | Cases | Comment |
|---|---|---|
| presentation_test.go `TestLoadPresentationProfile` | missing-file-defaults<br>de-non-developer-parsed<br>missing-front-matter-defaults<br>unknown-audience-is-developer<br>empty-language-defaults-en | table-driven, `t.TempDir()` fixtures |
| i18n_test.go `TestTranslate` | de-known-key<br>de-unknown-key-identity<br>en-passthrough | direct asserts (RULE-GOLANG-TEST-014 shape per case count) |
| server_test.go `TestNavNonDeveloper` | de profile fixture → body contains `Übersicht`/`Vorschläge`, lacks `/context`, `/repos`, `href="/scripts/skills"` | mirrors `TestNavPeekLink` (F9); `Options.PresentationPath` → fixture file |
| server_test.go `TestNavDefaultProfile` | no profile → body contains `Overview`, `/context`; `lang="en"` | pins the absent-profile invariant (D2) |
| server_test.go `TestLangAttrGerman` | de profile → `lang="de"` | |
| overview_test.go `TestOverviewNonDeveloper` | de profile → only session/proposal tiles rendered, no `Checklist`/`Repos` labels | extends `TestOverview` fixtures |
| proposals tests `TestProposalCardNonDeveloper` | card HTML lacks `band `, `→ `-target badge, tag badges; snippet summary is `Technisches Detail`; German placeholder | via `/proposals` with a fixture store |

- Existing tests as safety net: `TestNavPeekLink`, `TestOverview` run against the default (no-profile) path and must stay green untouched.
- Not tested: run.sh language threading and install.sh copy — the routine shell suite doesn't stage `claude` invocations; both are covered by Verification items instead.

## Test runbook

Scenario index (no `runbook` arg; verification tool is `curl` against the local configserver — the Browser pane is policy-blocked for 127.0.0.1):

- **german-nav** — `curl localhost:6001/` with de profile installed; expect `lang="de"`, `Übersicht`, no `/context` anchor.
- **default-unchanged** — same request, profile removed; expect today's English nav byte-wise.
- **card-internals-hidden** — `curl localhost:6001/proposals` with de profile; expect no `band`/target/tag badges, `Technisches Detail` present.
- **hook-injection** — run `cmd/hooks/global-context.sh` with a profile in `$HOME/.claude/context/global/`; expect the profile body in stdout.
- **nightly-prompt** — dry-derive: `sed` line from §9 against the de profile prints `de`.

## Contracts & sweeps

| Contract | Sides | Sweep |
|---|---|---|
| Profile front-matter keys (`language:`, `audience:`) | Go parser (presentation.go) · `sed` in run.sh · template file | grep `presentation-profile` to enumerate all readers; both parsers tested/verified against the same template file |
| `Options` struct gains `PresentationPath` | server.New · cmd/configserver/main.go · every `newTestServer` caller | build is the sweep (zero-value = defaults, no test churn) |
| FuncMap additions (`t`, `langAttr`, `isDeveloperAudience`) | server.go · all templates | `New` fails on any template referencing a missing func — boot in tests is the sweep |
| consolidate `language <lang>` arg | run.sh producer · smine-consolidate SKILL.md consumer | contract pre-exists (SKILL.md:32); no skill change |
| global-context injection format | hook · sessions consuming it | header lines unchanged; only a second source dir added |
| `-presentation-profile=<id>` flag | smine.iss `ProfileArg` · install.ps1 · main.go · runInstall (windows + stub) | grep `presentation-profile` across .iss/.ps1/Go; template file `presentation-profile.<id>.md` must exist for every id the wizard maps |

## Verification

- [ ] Run `make audit` — expect green (gofmt, vet, acdsl gates, tests).
- [ ] Run `go test ./internal/server/` — expect all new and existing tests green.
- [ ] Run `shellcheck cmd/hooks/global-context.sh` — expect no findings (ACDSL-SHELL-001).
- [ ] Boot `./bin/configserver` (built via `make build`) with `-presentation` pointing at a copy of `settings/claude_code/presentation-profile.de.md`; `curl -s localhost:6001/` — expect `lang="de"`, `Übersicht`, no `href="/context"`.
- [ ] Same boot, `curl -s localhost:6001/proposals` — expect no `band `, no target/tag badges, `Technisches Detail` on a card with snippets.
- [ ] Boot without `-presentation` file present; `curl -s localhost:6001/` — expect output identical to a pre-change build (English nav, full tabs).
- [ ] Degenerate: point `-presentation` at a file with no front matter — expect English/full UI, no error.
- [ ] Degenerate: profile with `language: fr` — expect `lang="fr"`, English text (identity fallback), full/reduced per audience.
- [ ] Run `HOME=$(mktemp -d) bash -c 'mkdir -p $HOME/.claude/context/global && cp settings/claude_code/presentation-profile.de.md $HOME/.claude/context/global/presentation-profile.md && bash cmd/hooks/global-context.sh'` from the worktree — expect the profile body on stdout under the GLOBAL CONTEXT header.
- [ ] Dry-check §9: `sed -n 's/^language:[[:space:]]*//p' settings/claude_code/presentation-profile.de.md | head -1` — expect `de`.
- [ ] Run `make installer-check` — expect the dockerized iscc compile of smine.iss (incl. the new ProfilePage/ProfileArg code) to pass; a tag run is never the first compile (F16).
- [ ] Run `go build ./cmd/configserver` with `GOOS=windows GOARCH=amd64` — expect install_windows.go (new param + copy helper) to compile.
- [ ] Not verifiable locally: the wizard radio page rendering and the end-to-end Windows install — verified on the target machine at rollout; the flag → copy path is covered by the cross-compile plus `installPresentationProfile` being plain stdlib file I/O.

## Stop conditions

| ID | Condition | Action |
|---|---|---|
| S1 | An approved signature/contract can't hold (ACTION-IMPL-001) | stop, report |
| S2 | Second failed approach in a row (ACTION-IMPL-002) | stop, re-read state, re-plan |
| S3 | Missing prerequisite needs infrastructure (ACTION-IMPL-003) | run the producing step; if infra is down, ask |
| S4 | Discovered work materially exceeds scope (ACTION-IMPL-004) | ask before continuing |
| S5 | Same bug class found twice (ACTION-IMPL-005) | fix all in-diff instances; report pre-existing |
| S6 | Structural obstacle tempts a new abstraction (ACTION-IMPL-006) | stop, report — relocate, don't indirect |
| S7 | A template references `t`/`isDeveloperAudience` but `New` fails to parse | stop — fix the FuncMap wiring, never fork templates |
| S8 | The catalog sweep (§4) surfaces user-visible strings only composable in Go (fmt.Sprintf composites) | leave English, record in Deferred findings — no Go-side string surgery in this pass |

## Changelog

| Date | Trigger | What changed |
|---|---|---|
| — | initial | plan created |
| 2026-08-24 | local: Windows installer must expose hard-coded profile selection | added D11, F14–F17, Changes §11 (flag + copy), §12 (wizard page), installer-check/cross-build verification, flag contract row |
| 2026-08-27 | local: reformat | Open questions moved after Decisions per updated RULE-PLAN-002 |
