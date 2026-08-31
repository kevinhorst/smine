GO           ?= go
GOOPTS       ?=
GOBUILD=$(GO) build $(GOOPTS)
GOTEST=$(GO) test -v -race $(GOOPTS)
VERSION      ?= 1.2.1
LDFLAGS      = -s -w -X main.version=$(VERSION)
ADDR         ?= :6001
SETTINGS     ?= $(HOME)/.claude/settings.json
ifeq ($(OS),Windows_NT)
BINEXT       := .exe
else
BINEXT       :=
endif


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


## build-audit: build the acdsl runner and every verifier binary (the gates spawn these instead of go run)
.PHONY: build-audit
build-audit:
	$(GOBUILD) -o ./bin/acdsl$(BINEXT) ./cmd/acdsl
	@mkdir -p ./bin/verifiers
	$(GOBUILD) -o ./bin/verifiers ./cmd/acdsl/verifiers/...

## audit: fast quality gate — vet, acdsl gates, tests without the race detector (release gate: audit-full)
.PHONY: audit
audit: build-audit
	go mod verify
	go vet ./...
	./bin/acdsl project -strip
	./bin/acdsl check
	./bin/acdsl fixtures
	go test -buildvcs -vet=off ./...

## audit-full: release gate — audit plus race-detector tests and the shell test suite
.PHONY: audit-full
audit-full: audit
	go test -race -buildvcs -vet=off ./...
	for t in cmd/tests/test_*.sh; do bash $$t || exit 1; done


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
	@echo "-> Building routinewrap ..."
	$(GOBUILD) -o ./bin/routinewrap$(BINEXT) ./cmd/routinewrap
	@echo "-> Building acdsl ..."
	$(GOBUILD) -o ./bin/acdsl$(BINEXT) ./cmd/acdsl
	@echo "-> Building rules ..."
	$(GOBUILD) -o ./bin/rules$(BINEXT) ./cmd/rules

## build: build all binaries and install
.PHONY: build-install
build-install: build
	./install.sh


## build-release: build release binaries for GOOS/GOARCH into bin/release (VERSION=x.y.z)
# The two daemons add -H=windowsgui on windows: no console window at logon.
GUIFLAGS = $(if $(filter windows,$(GOOS)),-H=windowsgui ,)
.PHONY: build-release
build-release:
	@echo "-> Building release binaries $(VERSION) for $(GOOS)/$(GOARCH) ..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(GUIFLAGS)$(LDFLAGS)' -o ./bin/release/smine-configserver-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,) ./cmd/configserver
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(GUIFLAGS)$(LDFLAGS)' -o ./bin/release/smine-routinewrap-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,) ./cmd/routinewrap
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(LDFLAGS)' -o ./bin/release/smine-acdsl-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,) ./cmd/acdsl
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(LDFLAGS)' -o ./bin/release/smine-rules-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,) ./cmd/rules


git-release:
	sed -i.bak 's/^VERSION = .*/VERSION = $(VERSION)/' Makefile
	sed -i.bak 's/^var version = ".*"/var version = "$(VERSION)"/' cmd/configserver/version.go
	rm -f Makefile.bak cmd/configserver/version.go.bak
	git commit -am "cmd: release v$(VERSION)"
	git tag v$(VERSION)

## run: build and run configserver
.PHONY: serve
serve: build
	@echo "-> Running configserver on $(ADDR) ..."
	./bin/configserver -addr $(ADDR) -settings $(SETTINGS)


## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...
	for t in cmd/tests/test_*.sh; do bash $$t || exit 1; done


## test-coverage: run all tests and display coverage
.PHONY: test-coverage
test-coverage:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage_configserver.out ./...
	go tool cover -html=/tmp/coverage_configserver.out


# ==================================================================================== #
# RELEASE
# ==================================================================================== #

## installer-check: compile smine.iss with a dockerized iscc against a dummy payload (recreates dist/)
# Run before touching CI: a tag run must never be the first iscc compile.
.PHONY: installer-check
installer-check:
	rm -rf dist
	mkdir -p dist/bin/verifiers
	for f in configserver routinewrap acdsl rules; do echo dummy > dist/bin/$$f.exe; done
	echo dummy > dist/bin/verifiers/dummy.exe
	echo dummy > dist/jq.exe
	echo dummy > dist/peek-mcp.exe
	mkdir -p dist/srctree
	echo dummy > dist/srctree/README.md
	docker run --rm --platform linux/amd64 -v "$$PWD:/work" amake/innosetup /DAppVersion=0.0.1 installer/windows/smine.iss
	rm -rf dist
