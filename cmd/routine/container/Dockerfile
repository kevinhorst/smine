# syntax=docker/dockerfile:1
# The image of a container routine's cells (plist ROUTINE_CONTAINER=1):
# the Linux rendering of what the host config execs — the claude CLI pinned to
# the host's version, the hook binaries and smine tools at the host-home paths
# the settings name, and the toolchain the make targets need. Built by the
# routine runner (internal/routinerun/container.go) from the main checkout;
# toolchain versions follow cmd/tests/linux/Dockerfile — change both together.
FROM golang:1.27-trixie

# trixie's git knows merge-tree --merge-base (agentwt pick, the worktree tests);
# bookworm's 2.39 does not. The main checkout's directory inside the cell is a
# root-owned mount parent holding the user's .git, which git's ownership check
# refuses — safe.directory lifts it for every path the cell can reach.
RUN apt-get update \
 && apt-get install -y --no-install-recommends build-essential ca-certificates curl git iptables jq rsync shellcheck socat \
 && rm -rf /var/lib/apt/lists/* \
 && git config --system safe.directory '*'

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
  | sh -s -- -y --profile minimal --component rustfmt,clippy

RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

# The cell runs as the host user at the host's home path: every bind mount
# keeps its absolute path and its owner. The dirs the mounts and named volumes
# land in are created here, so they take this owner.
ARG HOST_HOME
ARG HOST_UID
ARG HOST_GID
RUN (getent group ${HOST_GID} || groupadd --gid ${HOST_GID} cell) \
 && useradd --uid ${HOST_UID} --gid ${HOST_GID} --home-dir ${HOST_HOME} --no-create-home --shell /bin/bash cell \
 && mkdir -p ${HOST_HOME}/.claude/hooks/bin ${HOST_HOME}/go/bin ${HOST_HOME}/go/pkg/mod \
      ${HOST_HOME}/.cache/go-build ${HOST_HOME}/.cache/claude-routine /usr/local/cargo/registry \
 && chown -R ${HOST_UID}:${HOST_GID} ${HOST_HOME} /usr/local/cargo

ENV HOME=${HOST_HOME} \
    GOPATH=${HOST_HOME}/go \
    DISABLE_AUTOUPDATER=1 \
    PATH=${HOST_HOME}/go/bin:${HOST_HOME}/.local/bin:/go/bin:/usr/local/cargo/bin:$PATH

USER ${HOST_UID}:${HOST_GID}
ARG CLAUDE_VERSION
RUN curl -fsSL https://claude.ai/install.sh | bash -s ${CLAUDE_VERSION}

ARG PEEK_MCP_VERSION
ARG TARGETARCH
RUN curl -fsSL -o ${HOST_HOME}/go/bin/peek-mcp \
      https://github.com/kevinhorst/peek-mcp/releases/download/v${PEEK_MCP_VERSION}/peek-mcp-linux-${TARGETARCH} \
 && chmod +x ${HOST_HOME}/go/bin/peek-mcp

# The checkout's own binaries, built as root: the deployed hook set,
# smine-rules, smine-routine, smine-agentwt and smine-heartbeat. The cell runs
# smine-heartbeat only as a client of the host's one daemon, through the
# holder's port relay; serve refuses to start off macOS. The build caches are
# BuildKit cache mounts outside the home, so nothing root-owned lands where
# the cell's cache volumes mount;
# the hooks dir ends root-owned, so a cell cannot replace its own gate.
USER root
WORKDIR /src
ENV GOMODCACHE=/go/pkg/mod \
    GOCACHE=/root/.cache/go-build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG HOOKS
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    for name in ${HOOKS}; do \
      go build -o ${HOST_HOME}/.claude/hooks/bin/$name ./cmd/$name || exit 1; \
    done \
 && go build -o ${HOST_HOME}/go/bin/smine-rules ./cmd/rules \
 && go build -o ${HOST_HOME}/go/bin/smine-routine ./cmd/routine \
 && git config --system merge.skillcue.name "skill model changelog merge" \
 && git config --system merge.skillcue.driver "${HOST_HOME}/go/bin/smine-rules merge-skillcue %O %A %B %P %X %Y" \
 && chown -R root:root ${HOST_HOME}/.claude/hooks
RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/src/cmd/agentwt/target \
    cargo build --release --locked --manifest-path cmd/agentwt/Cargo.toml \
 && cp cmd/agentwt/target/release/agentwt ${HOST_HOME}/go/bin/smine-agentwt
RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/src/cmd/heartbeat/target \
    cargo build --release --locked --manifest-path cmd/heartbeat/Cargo.toml \
 && cp cmd/heartbeat/target/release/heartbeat ${HOST_HOME}/go/bin/smine-heartbeat
ENV GOMODCACHE= \
    GOCACHE=

USER ${HOST_UID}:${HOST_GID}
WORKDIR ${HOST_HOME}
