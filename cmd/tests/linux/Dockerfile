# The image of `make audit-linux`: the toolchain of CI's Linux jobs
# (.github/workflows/ci.yml, jobs test and audit) — change both together.
# The Rust toolchain is whatever stable is at build time, as on CI.
# Trixie, not bookworm: agentwt and the nightly runner need git >= 2.40
# (merge-tree --merge-base); bookworm ships 2.39, the CI runner a current git.
FROM golang:1.27-trixie

RUN apt-get update \
 && apt-get install -y --no-install-recommends build-essential ca-certificates curl git jq rsync shellcheck \
 && rm -rf /var/lib/apt/lists/*

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
  | sh -s -- -y --profile minimal --component rustfmt,clippy

# 2.14.0 is the first whose staticcheck (honnef.co/go/tools 0.8) builds Go 1.27's stdlib; 2.12.2 panics on it.
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

# Tests run as an unprivileged user, as on CI's runner: root reads a chmod 000
# file, so permission-denied cases would pass silently. The cache dirs the
# named volumes mount on are created here, so a fresh volume takes this owner.
RUN useradd --create-home --uid 1001 runner \
 && mkdir -p /home/runner/go/pkg/mod /home/runner/.cache/go-build /usr/local/cargo/registry /work \
 && chown -R runner:runner /home/runner /usr/local/cargo /work
USER runner
ENV GOPATH=/home/runner/go \
    PATH=/home/runner/go/bin:/go/bin:/usr/local/cargo/bin:$PATH

RUN git config --global init.defaultBranch main \
 && git config --global user.name test-linux \
 && git config --global user.email test-linux@localhost

WORKDIR /work

# /src is the checkout, mounted read-only. The copy leaves out what the host
# built and gets a one-commit repository — the shape of CI's shallow checkout;
# a linked worktree's .git file would point outside the container. Working-copy
# projection blocks are stripped before the snapshot commit, as before any
# real commit. The audit job runs first, then the test job; both always run,
# either failing fails.
CMD rsync -a \
      --exclude /.git --exclude /bin --exclude /.claude/worktrees \
      --exclude /cmd/agentwt/target --exclude /cmd/heartbeat/target \
      /src/ /work/ \
 && git init -q . && go run ./cmd/acdsl project -strip \
 && git add -A && git commit -q -m snapshot \
 && { echo '==== audit job'; make audit SCOPE=all; audit=$?; \
      echo '==== test job'; make test; test=$?; \
      echo "==== audit exit $audit, test exit $test"; [ "$audit" = 0 ] && [ "$test" = 0 ]; }
