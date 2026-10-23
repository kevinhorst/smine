GO           ?= go
GOOPTS       ?=
GOBUILD=$(GO) build $(GOOPTS)
GOTEST=$(GO) test -v -race $(GOOPTS)
VERSION      ?= 2.0.0
LDFLAGS      = -s -w -X main.version=$(VERSION)
ADDR         ?= :6001
SETTINGS     ?= $(HOME)/.claude/settings.json
ifeq ($(OS),Windows_NT)
BINEXT       := .exe
else
BINEXT       :=
endif
# cargo comes from PATH like go does (rustup's proxy dir is the shell's job — ~/.zshenv on this machine).
CARGO       ?= cargo
AGENTWT_MANIFEST = cmd/agentwt/Cargo.toml
HEARTBEAT_MANIFEST = cmd/heartbeat/Cargo.toml
RUST_CRATES  = agentwt heartbeat
TEST_LOG_DIR = bin/test-logs
# the audit phase runs go vet as its own unit, so the gated test runs skip go test's vet subset; make test clears it and keeps vet on
GOTEST_VET  ?= -vet=off
# A deadlocked test fails after this instead of go test's 10 m default, which held the calling agent for the whole wait
GOTEST_TIMEOUT ?= 5m
# SCOPE=changed runs the Rust crates whose files changed against the branch base, SCOPE=all both; BASE=<ref> overrides the derived base
SCOPE ?= changed
BASE  ?=
# the read-only gates of the concurrent phase; audit and audit-full add their test units
GATE_UNITS = logged-go-mod-verify logged-go-vet logged-cross-build logged-golden-check logged-workflow-gen-check logged-skills-check logged-acdsl-check logged-acdsl-fixtures logged-settings-defaults-check logged-fragment-check
# golangci-lint's cache and start-up lock (it locks in TMPDIR) stay per checkout — the same dirs golangciwrap sets
GOLANGCI_CACHE_DIR = $(CURDIR)/bin/golangci-cache
GOLANGCI_TMP_DIR   = $(CURDIR)/bin/golangci-tmp
# the release payload: GOOS/GOARCH default to the host, PAYLOAD_DIR to dist/<os>-<arch>; the Rust target follows them (darwin and linux)
GOOS        ?= $(shell $(GO) env GOOS)
GOARCH      ?= $(shell $(GO) env GOARCH)
PAYLOAD_DIR ?= dist/$(GOOS)-$(GOARCH)
PAYLOAD_EXE  = $(if $(filter windows,$(GOOS)),.exe,)
PAYLOAD_GO   = acdsl rules installer
HOOK_BINARIES = toolpolicy skillhelp worktreecreate sessiontitle readgate globalcontext skillcontext planmodecontext acdslhook skillinvokegate worktreewriteguard skillenv playsound heartbeatfeed
RUST_TARGET  = $(if $(filter arm64,$(GOARCH)),aarch64,x86_64)-$(if $(filter darwin,$(GOOS)),apple-darwin,unknown-linux-gnu)


# ==================================================================================== #
# HELPERS
# ==================================================================================== #


## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #


## build-audit: build the acdsl runner, the rules tool and every verifier binary without VCS stamps (the gates spawn these instead of go run; no stamp means a commit alone does not change the binaries)
.PHONY: build-audit
build-audit:
	$(GOBUILD) -buildvcs=false -o ./bin/acdsl$(BINEXT) ./cmd/acdsl
	$(GOBUILD) -buildvcs=false -o ./bin/rules$(BINEXT) ./cmd/rules
	@mkdir -p ./bin/verifiers
	$(GOBUILD) -buildvcs=false -o ./bin/verifiers ./cmd/acdsl/verifiers/...

## audit-prelude: the serial prefix of audit and audit-full — the three steps that write into the tree (build-audit, the projection strip, generate)
.PHONY: audit-prelude
audit-prelude: build-audit
	./bin/acdsl project -strip
	$(if $(wildcard context/aspects.json),$(MAKE) generate)

## go-mod-verify: go mod verify; a unit of the audit phase
.PHONY: go-mod-verify
go-mod-verify:
	go mod verify

## go-vet: go vet over the module; a unit of the audit phase
.PHONY: go-vet
go-vet:
	go vet ./...

## acdsl-check: the ACDSL gates, verdicts cached per checkout under bin/acdsl-cache; a unit of the audit phase
.PHONY: acdsl-check
acdsl-check:
	./bin/acdsl check -report bin/audit-report.md

## acdsl-fixtures: every rule's pass and fail fixtures; a unit of the audit phase
.PHONY: acdsl-fixtures
acdsl-fixtures:
	./bin/acdsl fixtures

## cross-build: compile every package for Windows and vet it for Linux, so a break on the OSes CI covers shows locally; part of audit
# The two forms CI proves: test-windows builds every package natively, the Linux audit job vets it. Windows test runs
# stay with CI — no Windows container runs on a Mac.
.PHONY: cross-build
cross-build:
	GOOS=windows GOARCH=amd64 $(GO) build ./...
	GOOS=linux GOARCH=amd64 $(GO) vet ./...

## audit-linux: run CI's two Linux jobs (make audit, then make test) in a container on a snapshot copy of this checkout, with CI's latest stable Rust — needs a running Docker daemon; not part of audit
# The checkout is mounted read-only and copied inside, so the host's bin/ and target/ dirs are never touched; the copy
# gets a one-commit repository, the shape CI's shallow checkout has. Both jobs always run and both report; the target
# fails when either does. Module, build and crate caches persist in named volumes.
.PHONY: audit-linux
audit-linux:
	docker build --pull -t smine-audit-linux cmd/tests/linux
	docker run --rm -v "$(CURDIR):/src:ro" -v smine-test-linux-runner-gomod:/home/runner/go/pkg/mod \
		-v smine-test-linux-runner-gocache:/home/runner/.cache/go-build -v smine-test-linux-runner-cargo:/usr/local/cargo/registry smine-audit-linux

## golden-check: generated per-environment golden files match their master (regenerate with go run ./cmd/skilleval goldens gen <master>)
.PHONY: golden-check
golden-check:
	@go run ./cmd/skilleval goldens check

## audit: fast quality gate — audit-prelude, then one concurrent phase of the gates, the Go tests without the race detector and the Rust crates whose files changed; SCOPE=all runs both crates, BASE=<ref> overrides the derived branch base (the release gate is audit-full)
.PHONY: audit
audit: audit-prelude
	$(MAKE) -j $(GATE_UNITS) logged-test-go $(addprefix logged-rust-crate-,$(shell ./bin/rules audit-crates -base "$(BASE)" -scope "$(SCOPE)" || echo agentwt heartbeat))

## audit-full: release gate — audit-prelude, then the gates, race tests, the changed Rust crates and the model queries concurrently, then the repo-mutating acdsl smoke alone; SCOPE=all runs both crates, BASE=<ref> overrides the base
.PHONY: audit-full
audit-full: audit-prelude
	$(MAKE) -j $(GATE_UNITS) logged-test-go-race $(addprefix logged-rust-crate-,$(shell ./bin/rules audit-crates -base "$(BASE)" -scope "$(SCOPE)" || echo agentwt heartbeat)) logged-model-queries
	$(MAKE) logged-test-acdsl-smoke

## logged-%: run target % with its output in bin/test-logs/%.log — one PASS line, or FAIL plus the whole log (make 3.81 has no --output-sync); the concurrent phases of audit, audit-full and test run every unit through it
logged-%:
	@mkdir -p $(TEST_LOG_DIR)
	@if $(MAKE) --no-print-directory $* > $(TEST_LOG_DIR)/$*.log 2>&1; then \
	  echo "PASS: $* ($(TEST_LOG_DIR)/$*.log)"; \
	else \
	  echo "FAIL: $* ($(TEST_LOG_DIR)/$*.log)"; \
	  cat $(TEST_LOG_DIR)/$*.log; \
	  exit 1; \
	fi


# ==================================================================================== #
# SEMANTIC MODEL
# ==================================================================================== #

SKILL ?= railroad-review

## model-analyze: full analysis of a skill's derived semantic model (SKILL=railroad-review) — clause checks and the z3 battery (consistency included), then every committed query
.PHONY: model-analyze
model-analyze:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "model-analyze: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	dir=$$(dirname "$$model"); \
	echo "== check =="; \
	$(GO) run ./cmd/skillmodel check "$$dir" || exit 1; \
	for query in "$$dir"/queries/*.json; do \
	  [ -e "$$query" ] || continue; \
	  echo "== query $$(basename $$query) =="; \
	  $(GO) run ./cmd/skillmodel solve --query "$$query" "$$dir" || exit 1; \
	done

WORKFLOW_DIRS := $(wildcard skills/*/*/workflows)

## workflow-gen: regenerate the GEN blocks (schemas + common prelude + model vocabulary) of every skill's stage workflow files
.PHONY: workflow-gen
workflow-gen:
	@$(GO) run ./cmd/rules workflow-gen $(WORKFLOW_DIRS)

## workflow-gen-check: fail when a stage workflow's GEN blocks are stale or its shape rules are violated; part of audit
.PHONY: workflow-gen-check
workflow-gen-check:
	@$(GO) run ./cmd/rules workflow-gen --check $(WORKFLOW_DIRS)

## skills-check: fail when a model file (<leaf>.cue) is not canonical, a model is invalid, a payload is orphaned, a generated, markdown-era or leftover skill.cue file survives or a dir holds a SKILL.md without a model file; part of audit
.PHONY: skills-check
skills-check:
	@$(GO) run ./cmd/skillmodel render -check skills

## model-queries: run every opted-in skill's z3 battery and committed queries — the model test suites; part of audit-full
.PHONY: model-queries
model-queries:
	@fail=0; \
	for dir in skills/*/*/queries; do \
	  [ -d "$$dir" ] || continue; \
	  skill="$$(dirname $$dir)"; \
	  echo "== $$skill =="; \
	  $(GO) run ./cmd/skillmodel check "$$skill" || fail=1; \
	  for query in "$$dir"/*.json; do \
	    [ -e "$$query" ] || continue; \
	    echo "== query $$(basename $$query) =="; \
	    $(GO) run ./cmd/skillmodel solve --query "$$query" "$$skill" || fail=1; \
	  done; \
	done; \
	exit $$fail

## model-diagram: whole-skill execution diagram as Mermaid (SKILL=railroad-review); JSON=1 prints the projection contract, PHASES=1 the execution chain only
.PHONY: model-diagram
model-diagram:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "model-diagram: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	flags=""; \
	if [ -n "$(JSON)" ]; then flags="--json"; fi; \
	if [ -n "$(PHASES)" ]; then flags="$$flags --phases"; fi; \
	$(GO) run ./cmd/skillmodel diagram $$flags "$$(dirname "$$model")"

## model-check: clause checks plus the z3 battery of a skill's derived model (SKILL=railroad-review) — one line per finding, exit 1 on an error finding; z3 needed when the skill sets tier: "strict"
.PHONY: model-check
model-check:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "model-check: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	$(GO) run ./cmd/skillmodel check "$$(dirname "$$model")"

## model-query: does a skill's model imply an expression? (SKILL=railroad-review EXPR='not asks_user') — unsat means it holds, with the proving invariants; sat prints a counterexample
.PHONY: model-query
model-query:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "model-query: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	if [ -z '$(EXPR)' ]; then echo "model-query: EXPR='<expr>' is required"; exit 2; fi; \
	$(GO) run ./cmd/skillmodel query --expr '$(EXPR)' "$$(dirname "$$model")"


## lint: run golangci-lint with the repo config (the same tool the acdsl golangci gate wraps) — per-checkout cache and lock, waits for another run in this checkout
.PHONY: lint
lint:
	@mkdir -p $(GOLANGCI_TMP_DIR)
	GOLANGCI_LINT_CACHE=$(GOLANGCI_CACHE_DIR) TMPDIR=$(GOLANGCI_TMP_DIR) golangci-lint run --allow-serial-runners ./...


## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v


# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #


## build: build all binaries
.PHONY: build
build:
	@echo "-> Building configserver ..."
	$(GOBUILD) -o ./bin/configserver$(BINEXT) ./cmd/configserver
	@echo "-> Building routine ..."
	$(GOBUILD) -o ./bin/routine$(BINEXT) ./cmd/routine
	@echo "-> Building acdsl ..."
	$(GOBUILD) -o ./bin/acdsl$(BINEXT) ./cmd/acdsl
	@echo "-> Building rules ..."
	$(GOBUILD) -o ./bin/rules$(BINEXT) ./cmd/rules
	$(MAKE) rust-build

## rust-build: build the agentwt and heartbeat binaries (release) into bin/
# Each binary is copied to a temp name and renamed into place: a fresh inode per install. cp over an existing file keeps
# its inode, and macOS SIGKILLs every later exec of a signed binary whose pages changed under a vnode it already ran
# (the launchd heartbeat daemon runs bin/heartbeat).
.PHONY: rust-build
rust-build:
	@echo "-> Building agentwt ..."
	$(CARGO) build --release --locked --manifest-path $(AGENTWT_MANIFEST)
	@mkdir -p ./bin
	cp cmd/agentwt/target/release/agentwt$(BINEXT) ./bin/.agentwt$(BINEXT).tmp
	mv -f ./bin/.agentwt$(BINEXT).tmp ./bin/agentwt$(BINEXT)
	@echo "-> Building heartbeat ..."
	$(CARGO) build --release --locked --manifest-path $(HEARTBEAT_MANIFEST)
	cp cmd/heartbeat/target/release/heartbeat$(BINEXT) ./bin/.heartbeat$(BINEXT).tmp
	mv -f ./bin/.heartbeat$(BINEXT).tmp ./bin/heartbeat$(BINEXT)

## rust-lock: (re)generate cmd/agentwt/Cargo.lock and cmd/heartbeat/Cargo.lock — the only target that touches the locks; every other rust target runs --locked
.PHONY: rust-lock
rust-lock:
	$(CARGO) generate-lockfile --manifest-path $(AGENTWT_MANIFEST)
	$(CARGO) generate-lockfile --manifest-path $(HEARTBEAT_MANIFEST)

## rust-test: agentwt and heartbeat unit and integration tests
.PHONY: rust-test
rust-test: $(addprefix rust-test-,$(RUST_CRATES))

## rust-test-%: one crate's unit and integration tests (rust-test-agentwt, rust-test-heartbeat)
rust-test-%:
	$(CARGO) test --locked --manifest-path cmd/$*/Cargo.toml

## rust-load: load evidence — 40-branch synthetic repo (always), plus REPO=<clone> and OLD=<checkout with the old scripts> for the real-repo comparison
.PHONY: rust-load
rust-load:
	AGENTWT_LOAD_REPO=$(REPO) AGENTWT_LOAD_OLD=$(OLD) $(CARGO) test --release --locked --manifest-path $(AGENTWT_MANIFEST) --test integration -- --ignored --nocapture load_

## heartbeat-load: load evidence — 2,000 hook events/s for 60 s through the real daemon; exits 1 when a target of exploration D4 is missed
.PHONY: heartbeat-load
heartbeat-load:
	$(CARGO) bench --locked --manifest-path $(HEARTBEAT_MANIFEST) --bench load

## rust-fmt: apply rustfmt to the agentwt and heartbeat crates (rust-lint only checks)
.PHONY: rust-fmt
rust-fmt:
	$(CARGO) fmt --manifest-path $(AGENTWT_MANIFEST)
	$(CARGO) fmt --manifest-path $(HEARTBEAT_MANIFEST)

## rust-lint: rustfmt check + clippy with warnings denied + lib-only check (the agentwt and heartbeat gate slice)
.PHONY: rust-lint
rust-lint: $(addprefix rust-lint-,$(RUST_CRATES))

## rust-lint-%: one crate's rustfmt check, clippy with warnings denied and lib-only check (rust-lint-agentwt, rust-lint-heartbeat)
rust-lint-%:
	$(CARGO) fmt --manifest-path cmd/$*/Cargo.toml -- --check
	$(CARGO) clippy --locked --manifest-path cmd/$*/Cargo.toml --all-targets --all-features -- -D warnings
	$(CARGO) check --locked --manifest-path cmd/$*/Cargo.toml --lib --no-default-features

## rust-crate-%: one crate's lint, then its tests (rust-crate-agentwt, rust-crate-heartbeat) — crates build into separate target dirs and run side by side; lint and tests of one crate share the target-dir lock and stay serial
rust-crate-%: rust-lint-%
	$(MAKE) --no-print-directory rust-test-$*

## build-install: [operator] build all binaries and install
.PHONY: build-install
build-install: build
	./install.sh

## install-full: [operator] install every component and all skills without prompts — the installer builds the binaries itself
.PHONY: install-full
install-full:
	go run ./cmd/installer -components full -skills all -yes


## payload-bin: cross-build every Go binary of a release payload into $(PAYLOAD_DIR)/bin and write its PAYLOAD marker — GOOS=darwin GOARCH=arm64 VERSION=x.y.z PAYLOAD_DIR=dist
# The two daemons add -H=windowsgui on windows: no console window at logon. The marker makes the installer and the apply
# engine take bin/<name> instead of building (internal/install/payload.go).
GUIFLAGS = $(if $(filter windows,$(GOOS)),-H=windowsgui ,)
.PHONY: payload-bin
payload-bin:
	@echo "-> Building payload $(VERSION) for $(GOOS)/$(GOARCH) into $(PAYLOAD_DIR)/bin ..."
	@mkdir -p $(PAYLOAD_DIR)/bin/verifiers
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(GUIFLAGS)$(LDFLAGS)' -o $(PAYLOAD_DIR)/bin/configserver$(PAYLOAD_EXE) ./cmd/configserver
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(GUIFLAGS)$(LDFLAGS)' -o $(PAYLOAD_DIR)/bin/routine$(PAYLOAD_EXE) ./cmd/routine
	for name in $(PAYLOAD_GO) $(HOOK_BINARIES); do GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(LDFLAGS)' -o $(PAYLOAD_DIR)/bin/$$name$(PAYLOAD_EXE) ./cmd/$$name || exit 1; done
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(LDFLAGS)' -o $(PAYLOAD_DIR)/bin/verifiers ./cmd/acdsl/verifiers/...
	echo $(VERSION) > $(PAYLOAD_DIR)/bin/PAYLOAD

## payload-rust: build agentwt and heartbeat for the payload's RUST_TARGET into $(PAYLOAD_DIR)/bin — GOOS=darwin GOARCH=arm64 PAYLOAD_DIR=dist; darwin and linux, the windows job builds agentwt.exe and heartbeat.exe natively
.PHONY: payload-rust
payload-rust:
	$(CARGO) build --release --locked --manifest-path $(AGENTWT_MANIFEST) --target $(RUST_TARGET)
	cp cmd/agentwt/target/$(RUST_TARGET)/release/agentwt $(PAYLOAD_DIR)/bin/agentwt
	$(CARGO) build --release --locked --manifest-path $(HEARTBEAT_MANIFEST) --target $(RUST_TARGET)
	cp cmd/heartbeat/target/$(RUST_TARGET)/release/heartbeat $(PAYLOAD_DIR)/bin/heartbeat

## payload: the release archive dist/smine-<version>-<os>-<arch>.tar.gz — GOOS=darwin GOARCH=arm64 VERSION=x.y.z; the source tree at HEAD plus payload-bin and payload-rust under smine/bin; extract, then run smine/bin/installer
.PHONY: payload
payload: payload-bin payload-rust
	rm -rf $(PAYLOAD_DIR)/smine
	mkdir -p $(PAYLOAD_DIR)/smine
	git archive --format=tar HEAD | tar -x -C $(PAYLOAD_DIR)/smine
	cp -R $(PAYLOAD_DIR)/bin $(PAYLOAD_DIR)/smine/bin
	tar -C $(PAYLOAD_DIR) -czf dist/smine-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz smine
	@echo "-> dist/smine-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz"


git-release:
	sed -i.bak 's/^VERSION = .*/VERSION = $(VERSION)/' Makefile
	sed -i.bak 's/^var version = ".*"/var version = "$(VERSION)"/' cmd/configserver/version.go
	rm -f Makefile.bak cmd/configserver/version.go.bak
	git commit -am "cmd: release v$(VERSION)"
	git tag v$(VERSION)

## docs-deploy: [operator] deploy docs/ to Firebase hosting (smine.info) by hand — every release tag redeploys it via CI
.PHONY: docs-deploy
docs-deploy:
	firebase deploy --only hosting

## serve: [operator] build and run configserver against the live settings
.PHONY: serve
serve: build
	@echo "-> Running configserver on $(ADDR) ..."
	./bin/configserver -addr $(ADDR) -settings $(SETTINGS)


## test: run all tests — Go race tests (vet on) and each Rust crate's tests concurrently, then the acdsl smoke alone
.PHONY: test
test:
	$(MAKE) -j GOTEST_VET= logged-test-go-race $(addprefix logged-rust-test-,$(RUST_CRATES))
	$(MAKE) logged-test-acdsl-smoke

## test-go: Go tests without the race detector
.PHONY: test-go
test-go:
	go test -timeout $(GOTEST_TIMEOUT) $(GOTEST_VET) ./...

## test-go-race: Go tests with the race detector
.PHONY: test-go-race
test-go-race:
	go test -race -timeout $(GOTEST_TIMEOUT) $(GOTEST_VET) ./...

## test-acdsl-smoke: the acdsl smoke (cmd/acdsl/smoke_test.go, build tag acdslsmoke) alone — it projects, stages and strips real tracked files, so nothing may run beside it
.PHONY: test-acdsl-smoke
test-acdsl-smoke:
	go test -tags acdslsmoke -run TestSmoke -count=1 -v -timeout $(GOTEST_TIMEOUT) ./cmd/acdsl/


## test-coverage: run all tests and display coverage
.PHONY: test-coverage
test-coverage:
	go test -v -race -buildvcs -timeout $(GOTEST_TIMEOUT) -coverprofile=/tmp/coverage_configserver.out ./...
	go tool cover -html=/tmp/coverage_configserver.out


# ==================================================================================== #
# AGENT ENTRY POINTS — the only non-toolchain commands an agent runs (tool-policy gate:
# any target, plus the NAME= parameters each target's "## <target>:" help line declares)
# ==================================================================================== #

## acdsl: run the acdsl runner from source — ARGS="check" | "check -rule <id>" | "fixtures" | "project -strip" | "project -plan <path>" | "verdicts -since 720h"
.PHONY: acdsl
acdsl:
	@go run ./cmd/acdsl $(ARGS)

## rules: run the rules tool from source — ARGS="generate" | "intake --base main --head HEAD" | "render-skill --strip-formal <skill dir>"
.PHONY: rules
rules:
	@go run ./cmd/rules $(ARGS)

## skill-render: load and check one skill's model and re-save its model file (<leaf>.cue) and payload files canonically — SKILL=<leaf>
.PHONY: skill-render
skill-render:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "skill-render: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	$(GO) run ./cmd/skillmodel render "$$(dirname "$$model")"

## skill-export: print one skill's loaded model with its merged atoms as JSON, for jq — SKILL=<leaf>
.PHONY: skill-export
skill-export:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "skill-export: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	$(GO) run ./cmd/skillmodel export "$$(dirname "$$model")"

## skill-atoms: print one skill's atom census — per atom its kind, source, readers by entry and role, and query names — SKILL=<leaf>
.PHONY: skill-atoms
skill-atoms:
	@model=$$(ls skills/*/$(SKILL)/$(SKILL).cue skills/$(SKILL)/$(SKILL).cue 2>/dev/null | head -1); \
	if [ -z "$$model" ]; then echo "skill-atoms: no $(SKILL).cue for skill '$(SKILL)'"; exit 2; fi; \
	$(GO) run ./cmd/skillmodel atoms "$$(dirname "$$model")"

## skill-bundle: write the standalone skill bundle to dist/skills — every skill that requires no component, rendered for the empty component set; fails on a smine reference
.PHONY: skill-bundle
skill-bundle:
	@rm -rf dist/skills
	@$(GO) run ./cmd/skillmodel bundle dist/skills

## skill-import: import a markdown skill dir (SKILL.md, atoms.json, changelog.json) or a vocabulary.json into the model; never deletes — DIR=<path>
.PHONY: skill-import
skill-import:
	@if [ -z "$(DIR)" ]; then echo "skill-import: DIR=<skill dir or vocabulary.json> is required"; exit 2; fi
	@$(GO) run ./cmd/skillmodel import $(DIR)

## skill-smoke: run one skill's smoke on its dummy input and print the verdict — SKILL=<leaf> TOKEN=<label>
.PHONY: skill-smoke
skill-smoke:
	@$(GO) run ./cmd/skilleval smoke -token "$(TOKEN)" $(SKILL)

## skill-smoke-verdict: judge the session of an interactive smoke — SKILL=<leaf> SESSION=<id>
.PHONY: skill-smoke-verdict
skill-smoke-verdict:
	@$(GO) run ./cmd/skilleval smoke-verdict $(SKILL) $(SESSION)

## generate: regenerate the untracked context/context.json from the entry docs and context/aspects.json — shorthand for make rules ARGS="generate"; audit runs it before the gates
.PHONY: generate
generate:
	@go run ./cmd/rules generate

## settings-defaults: regenerate cmd/heartbeat/tests/fixtures/config-defaults.json, the heartbeat defaults the Rust accessors are checked against
.PHONY: settings-defaults
settings-defaults:
	@go run ./cmd/rules settings-defaults cmd/heartbeat/tests/fixtures/config-defaults.json

## settings-defaults-check: fail when the heartbeat defaults fixture is stale against the settings schema; a unit of the audit phase
.PHONY: settings-defaults-check
settings-defaults-check:
	@go run ./cmd/rules settings-defaults --check cmd/heartbeat/tests/fixtures/config-defaults.json

## fragment-check: fail when a settings fragment hook group changed against the branch base without a smineVersion bump — the apply keeps an equal-version live group's command; BASE=<ref> overrides the derived base; a unit of the audit phase
.PHONY: fragment-check
fragment-check:
	@go run ./cmd/rules fragment-check -base "$(BASE)"

## sessions: smine session records — ARGS="pending|render|mark|unsynthesized …"
.PHONY: sessions
sessions:
	@go run ./cmd/sessions $(ARGS)

## style-check: scan the learned style profile for secrets/PII; a hit replaces it with the withheld stub and exits 1 — ARGS="[--path <file>]"
.PHONY: style-check
style-check:
	@go run ./cmd/stylecheck $(ARGS)

## context-record: used-context record of one Claude session and its subagents — ARGS="<transcript.jsonl> [ctx-dir]"
.PHONY: context-record
context-record:
	@go run ./cmd/sessions context-record $(ARGS)

## serve-dev: build, start a sandboxed configserver for this checkout detached (macOS, loopback, no credential) and return once it listens, printing its port; it runs until make serve-dev-stop and a second start is refused while it does; log in bin/dev/serve-dev.log; DEV_PORT=<n> fixes the port (default 0: a free port); DEV_CLAUDE_HOME=<dir> reads another Claude home, e.g. ~/.claude, its acdsl/verdicts.jsonl included; Claude token counting follows routines.count_tokens in that home's install record
override DEV_ROOT := $(CURDIR)/bin/dev
override DEV_HOME := $(DEV_ROOT)/home
override DEV_PID_FILE := $(DEV_ROOT)/serve-dev.pid
override DEV_LOG := $(DEV_ROOT)/serve-dev.log
override DEV_LISTENING := Config server listening on
DEV_PORT        ?= 0
DEV_CLAUDE_HOME ?= $(DEV_HOME)/.claude

# Security boundary of the dev server. It lives here, not in Go, because make
# sources are write-locked for agents and Go files are not. Rules, in order:
#   1. files: writes only under bin/dev
#   2. no launchctl (launchd is per user, not per HOME), no osascript dialogs
#   3. no signals to processes outside the sandbox
#   4. no connections to loopback services (heartbeat, peek, the operator's server)
override define DEV_SANDBOX
(version 1)
(allow default)
(deny file-write*)
(allow file-write* (subpath "$(DEV_ROOT)") (literal "/dev/null"))
(deny process-exec (literal "/bin/launchctl") (literal "/usr/bin/osascript"))
(deny signal)
(allow signal (target same-sandbox))
(deny network-outbound (remote ip "localhost:*"))
endef
export DEV_SANDBOX

# nohup execs the sandbox, which execs the server: one process, one pid ($!) — the one serve-dev-stop signals. The
# recipe returns on the listening line, so a caller never waits on a task that does not end; no line within 60 s, or
# an early exit, prints the log and fails.
.PHONY: serve-dev
serve-dev: build
	@mkdir -p $(DEV_ROOT)/tmp
	@pid=$$(cat $(DEV_PID_FILE) 2>/dev/null); \
	if [ -n "$$pid" ] && ps -p "$$pid" -o command= | grep -q "configserver -dev"; then \
	  echo "serve-dev: dev server $$pid is already running — make serve-dev-stop first"; \
	  grep "$(DEV_LISTENING)" $(DEV_LOG); \
	  exit 1; \
	fi; \
	HOME=$(DEV_HOME) TMPDIR=$(DEV_ROOT)/tmp nohup sandbox-exec -p "$$DEV_SANDBOX" \
		./bin/configserver -dev -addr 127.0.0.1:$(DEV_PORT) \
		-claude-home $(DEV_CLAUDE_HOME) -acdsl-verdicts $(DEV_CLAUDE_HOME)/acdsl/verdicts.jsonl \
		> $(DEV_LOG) 2>&1 < /dev/null & \
	pid=$$!; \
	echo $$pid > $(DEV_PID_FILE); \
	tries=0; \
	until grep -q "$(DEV_LISTENING)" $(DEV_LOG); do \
	  if ! kill -0 $$pid 2>/dev/null; then \
	    rm -f $(DEV_PID_FILE); \
	    echo "serve-dev: dev server exited before listening — $(DEV_LOG):"; \
	    cat $(DEV_LOG); \
	    exit 1; \
	  fi; \
	  tries=$$((tries + 1)); \
	  if [ $$tries -ge 600 ]; then \
	    kill $$pid; \
	    rm -f $(DEV_PID_FILE); \
	    echo "serve-dev: no listening line within 60 s, stopped $$pid — $(DEV_LOG):"; \
	    cat $(DEV_LOG); \
	    exit 1; \
	  fi; \
	  sleep 0.1; \
	done; \
	cat $(DEV_LOG); \
	echo "serve-dev: dev server $$pid runs detached — make serve-dev-stop ends it"

## serve-dev-stop: stop the dev configserver this checkout's last make serve-dev started, a no-op with a message when none is recorded or it already exited; then drop the Go module cache a sandboxed apply left under bin/dev/home (read-only dirs git worktree remove cannot delete)
# The recorded pid is signalled only while it still is a dev configserver, so a pid the system reused is left alone.
.PHONY: serve-dev-stop
serve-dev-stop:
	@pid=$$(cat $(DEV_PID_FILE) 2>/dev/null); \
	if [ -z "$$pid" ]; then echo "serve-dev-stop: no dev server recorded for this checkout"; \
	elif ! ps -p "$$pid" -o command= | grep -q "configserver -dev"; then \
	  rm -f $(DEV_PID_FILE); echo "serve-dev-stop: recorded dev server $$pid is not running"; \
	else \
	  rm -f $(DEV_PID_FILE); kill "$$pid" || exit 1; echo "serve-dev-stop: stopped dev server $$pid"; \
	fi; \
	if [ -d $(DEV_HOME)/go/pkg/mod ]; then \
	  GOMODCACHE=$(DEV_HOME)/go/pkg/mod go clean -modcache && echo "serve-dev-stop: dropped $(DEV_HOME)/go/pkg/mod"; \
	fi


# ==================================================================================== #
# RELEASE
# ==================================================================================== #

## redact: apply the sed ERE script SCRIPT= to every tracked text file outside vendor/ — release-sync redaction helper
# Binary files (git grep -I) and vendor/ are skipped; the script file itself stays outside the repo.
.PHONY: redact
redact:
	@test -f "$(SCRIPT)" || { echo "redact: SCRIPT= must name a sed script file"; exit 1; }
	git grep -I -l -e '' -- . ':!vendor' | xargs sed -i '' -E -f $(SCRIPT)

## installer-check: compile smine.iss with a dockerized iscc against a dummy payload (recreates dist/)
# Run before touching CI: a tag run must never be the first iscc compile.
.PHONY: installer-check
installer-check:
	rm -rf dist
	mkdir -p dist/bin/verifiers
	for f in configserver routine agentwt heartbeat $(PAYLOAD_GO) $(HOOK_BINARIES); do echo dummy > dist/bin/$$f.exe; done
	echo dummy > dist/bin/verifiers/dummy.exe
	echo 0.0.1 > dist/bin/PAYLOAD
	echo dummy > dist/jq.exe
	echo dummy > dist/peek-mcp.exe
	mkdir -p dist/srctree
	echo dummy > dist/srctree/README.md
	docker run --rm --platform linux/amd64 -v "$$PWD:/work" amake/innosetup /DAppVersion=0.0.1 installer/windows/smine.iss
	rm -rf dist
