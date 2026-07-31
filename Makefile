GO           ?= go
GOOPTS       ?=
GOBUILD=$(GO) build $(GOOPTS)
GOTEST=$(GO) test -v -race $(GOOPTS)
VERSION      ?= 1.0.1
LDFLAGS      = -s -w -X main.version=$(VERSION)
ADDR         ?= :6001
SETTINGS     ?= $(HOME)/.claude/settings.json


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


## audit: run quality control checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run ./cmd/rules validate
	go run ./cmd/rules generate -check
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
	$(GOBUILD) -o ./bin/configserver ./cmd/configserver
	@echo "-> Building secretscan ..."
	$(GOBUILD) -o ./bin/secretscan ./cmd/secretscan


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


## build-release: build release binaries for GOOS/GOARCH into bin/release (VERSION=x.y.z)
.PHONY: build-release
build-release:
	@echo "-> Building release binaries $(VERSION) for $(GOOS)/$(GOARCH) ..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(LDFLAGS)' -o ./bin/release/smine-configserver-$(GOOS)-$(GOARCH) ./cmd/configserver
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -ldflags '$(LDFLAGS)' -o ./bin/release/smine-secretscan-$(GOOS)-$(GOARCH) ./cmd/secretscan
