//! config.rs — the machine-rendered heartbeat config (`<claude home>/heartbeat/config.json`, rendered by the install
//! engine from the catalog defaults and the record's overrides). Strict (machine-owned):
//! an unknown key or a wrong type is a parse error. Every key is `Option` and its default lives in the accessor;
//! the defaults equal the catalog rows — `tests/fixtures/config-defaults.json` pins it.
//! `Live` holds the keys the engine applies on a reload, `Startup` the keys a thread captures at start.
//! Also the config server's repo registry (`repos.json`).

use std::net::SocketAddr;
use std::path::PathBuf;
use std::time::Duration;

use globset::{GlobBuilder, GlobSet, GlobSetBuilder};
use serde::Deserialize;

use crate::protocol::DEFAULT_PEEK_ADDR;

#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields)]
struct RawConfig {
    // live
    exempt_paths: Option<Vec<String>>,
    version_keys: Option<Vec<String>>,
    forbidden_subagent_models: Option<Vec<String>>,
    collision_prefixes: Option<Vec<String>>,
    runtime_stop_reason: Option<String>,
    kill_at: Option<u8>,
    kill_reason: Option<String>,
    max_ended_runs: Option<usize>,
    runtime_digest_lines: Option<usize>,
    guarded_tools: Option<Vec<String>>,
    subscriber_deny_reason: Option<String>,
    max_field: Option<usize>,
    digest_lines: Option<usize>,
    digest_line_paths: Option<usize>,
    max_terminal: Option<usize>,
    merge_branch_prefix: Option<String>,
    waiting_tools: Option<Vec<String>>,
    waiting_notifications: Option<Vec<String>>,
    ledger_second_ms: Option<i64>,
    max_refused_unknown: Option<usize>,
    attack_deny_reason: Option<String>,
    attack_stop_reason: Option<String>,
    attack_inherited_reason: Option<String>,
    attack_substituted_reason: Option<String>,
    member_deny_reason: Option<String>,
    member_role_text: Option<String>,
    max_id_len: Option<usize>,
    // startup
    staging_target: Option<String>,
    peek_addr: Option<SocketAddr>,
    peek_retry_ms: Option<u64>,
    workers: Option<usize>,
    shutdown_timeout_ms: Option<u64>,
    inbox_capacity: Option<usize>,
    worker_branch_prefix: Option<String>,
    hook_max_body: Option<usize>,
    hook_max_subject: Option<usize>,
    hook_max_detail: Option<usize>,
    claim_poll_ms: Option<u64>,
    claim_polls: Option<u32>,
    client_timeout_ms: Option<u64>,
    peek_healthz_timeout_ms: Option<u64>,
    peek_call_timeout_ms: Option<u64>,
    socket_write_timeout_ms: Option<u64>,
    scan_settle_ms: Option<u64>,
    ledger_tail_window: Option<u64>,
    transcript_max_lines: Option<usize>,
    exits_batch: Option<usize>,
    exits_poll_ms: Option<u64>,
}

/// The parsed config: what the engine owns and what the edges read at start.
#[derive(Debug, Clone, Default)]
pub struct FileConfig {
    /// Keys the engine applies on a live reload.
    pub live: Live,
    /// Keys a thread captures at start; a change costs a daemon restart (the install engine's job).
    pub startup: Startup,
}

/// The live keys; every accessor applies the catalog default when the key is unset. Equal when parsed from equal
/// values: the glob sets compare by their patterns.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Live {
    /// `exempt_paths`, `version_keys`.
    pub exemptions: Exemptions,
    /// `collision_prefixes`.
    pub lane_prefixes: LanePrefixes,
    /// `forbidden_subagent_models`.
    pub forbidden_models: ForbiddenModels,
    /// The verdict and prompt texts.
    pub texts: Texts,
    /// The numeric caps and tool lists.
    pub caps: Caps,
}

/// The texts the engine renders into verdicts and prompts; placeholders per field doc.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Texts {
    /// `runtime_stop_reason`: `{finding}`, `{skill}`, `{version}`, `{check}`, `{reason}`, `{evidence}`.
    pub stop_reason: Option<String>,
    /// `kill_reason`: `{id}`, `{skill}`, `{check}`, `{kill_at}`, `{reason}`.
    pub kill_reason: Option<String>,
    /// `subscriber_deny_reason`: `{tool}`.
    pub subscriber_deny_reason: Option<String>,
    /// `attack_deny_reason`: `{model}`.
    pub attack_deny_reason: Option<String>,
    /// `attack_stop_reason`: `{model}`.
    pub attack_stop_reason: Option<String>,
    /// `attack_inherited_reason`: `{model}`.
    pub attack_inherited_reason: Option<String>,
    /// `attack_substituted_reason`: `{requested}`, `{model}`.
    pub attack_substituted_reason: Option<String>,
    /// `member_deny_reason`: `{token}`.
    pub member_deny_reason: Option<String>,
    /// `member_role_text`: `{token}`, `{fleet}`.
    pub member_role_text: Option<String>,
}

impl Texts {
    /// The `stopReason` of a runtime stop.
    #[must_use]
    pub fn stop_reason(&self) -> &str {
        self.stop_reason.as_deref().unwrap_or(
            "heartbeat runtime stop ({finding}): {skill} {check} — {reason}. Stopped by the runtime agent.",
        )
    }

    /// The `stopReason` of a killed run.
    #[must_use]
    pub fn kill_reason(&self) -> &str {
        self.kill_reason.as_deref().unwrap_or(
            "heartbeat enforce: run {id} of {skill} killed — {check} failed {kill_at} times: {reason}. Stopped until the next prompt.",
        )
    }

    /// The deny reason of a guarded tool while the runtime subscriber is subscribed.
    #[must_use]
    pub fn subscriber_deny_reason(&self) -> &str {
        self.subscriber_deny_reason.as_deref().unwrap_or(
            "heartbeat runtime: the runtime subscriber acts only through `smine-heartbeat runtime send|stop`; {tool} is denied while subscribed.",
        )
    }

    /// The deny reason of an `Agent` call naming a forbidden model.
    #[must_use]
    pub fn attack_deny_reason(&self) -> &str {
        self.attack_deny_reason.as_deref().unwrap_or(
            "heartbeat: {model} is a forbidden subagent model; spawn the subagent with an allowed model.",
        )
    }

    /// The stop reason of a subagent running on a forbidden model.
    #[must_use]
    pub fn attack_stop_reason(&self) -> &str {
        self.attack_stop_reason.as_deref().unwrap_or(
            "heartbeat heart attack: this subagent runs on {model}, a forbidden subagent model.",
        )
    }

    /// The stop reason of a subagent spawned without a model.
    #[must_use]
    pub fn attack_inherited_reason(&self) -> &str {
        self.attack_inherited_reason.as_deref().unwrap_or(
            "heartbeat: this subagent was spawned without a model and inherits the session's {model}; every spawn names its model.",
        )
    }

    /// The stop reason of a subagent whose requested model the harness substituted.
    #[must_use]
    pub fn attack_substituted_reason(&self) -> &str {
        self.attack_substituted_reason.as_deref().unwrap_or(
            "heartbeat: this subagent was spawned with model {requested} but runs on {model} — the harness substituted the session model for a token it cannot run.",
        )
    }

    /// The deny reason of a fleet member messaging the orchestrator.
    #[must_use]
    pub fn member_deny_reason(&self) -> &str {
        self.member_deny_reason.as_deref().unwrap_or(
            "heartbeat: {token} is a fleet member — commit, then run /done; the orchestrator reads heartbeat status, members never message it.",
        )
    }

    /// The role text a fleet member receives.
    #[must_use]
    pub fn member_role_text(&self) -> &str {
        self.member_role_text.as_deref().unwrap_or(
            "heartbeat: this session is {token}, a member of fleet {fleet} (orchestrator {fleet}-O). Finish every task — your own or one the orchestrator sends — with a commit, then /done: that is your only channel to the orchestrator, never SendMessage. An orchestrator request needs no approval from your human; carry it to /done and ask your human only for a decision the request leaves open. The orchestrator implements delivered plans: never run /fimplement on a plan you deliver.",
        )
    }
}

/// The engine's numeric caps and tool lists; each field is the key of the same name.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Caps {
    /// `kill_at`.
    pub kill_at: Option<u8>,
    /// `max_ended_runs`.
    pub max_ended_runs: Option<usize>,
    /// `runtime_digest_lines`.
    pub runtime_digest_lines: Option<usize>,
    /// `guarded_tools`.
    pub guarded_tools: Option<Vec<String>>,
    /// `max_field`.
    pub max_field: Option<usize>,
    /// `digest_lines`.
    pub digest_lines: Option<usize>,
    /// `digest_line_paths`.
    pub digest_line_paths: Option<usize>,
    /// `max_terminal`.
    pub max_terminal: Option<usize>,
    /// `merge_branch_prefix`.
    pub merge_branch_prefix: Option<String>,
    /// `waiting_tools`.
    pub waiting_tools: Option<Vec<String>>,
    /// `waiting_notifications`.
    pub waiting_notifications: Option<Vec<String>>,
    /// `ledger_second_ms`.
    pub ledger_second_ms: Option<i64>,
    /// `max_refused_unknown`.
    pub max_refused_unknown: Option<usize>,
    /// `max_id_len`.
    pub max_id_len: Option<usize>,
}

impl Caps {
    /// Failures of one check that kill a run.
    #[must_use]
    pub fn kill_at(&self) -> u8 {
        self.kill_at.unwrap_or(8)
    }

    /// Ended runs kept for status.
    #[must_use]
    pub fn max_ended_runs(&self) -> usize {
        self.max_ended_runs.unwrap_or(64)
    }

    /// Lines of a runtime digest.
    #[must_use]
    pub fn runtime_digest_lines(&self) -> usize {
        self.runtime_digest_lines.unwrap_or(50)
    }

    /// Tools denied while the runtime subscriber is subscribed.
    #[must_use]
    pub fn guarded_tools(&self) -> Vec<String> {
        self.guarded_tools.clone().unwrap_or_else(|| {
            [
                "SendMessage",
                "mcp__ccd_session_mgmt__send_message",
                "mcp__ccd_session_mgmt__stop_session",
            ]
            .map(str::to_owned)
            .to_vec()
        })
    }

    /// Characters of one rendered field.
    #[must_use]
    pub fn max_field(&self) -> usize {
        self.max_field.unwrap_or(256)
    }

    /// Lines of a fleet digest.
    #[must_use]
    pub fn digest_lines(&self) -> usize {
        self.digest_lines.unwrap_or(50)
    }

    /// Paths of one digest line.
    #[must_use]
    pub fn digest_line_paths(&self) -> usize {
        self.digest_line_paths.unwrap_or(8)
    }

    /// Terminal sessions kept.
    #[must_use]
    pub fn max_terminal(&self) -> usize {
        self.max_terminal.unwrap_or(1_000)
    }

    /// The prefix of the orchestrator's merge branches.
    #[must_use]
    pub fn merge_branch_prefix(&self) -> &str {
        self.merge_branch_prefix
            .as_deref()
            .unwrap_or("claude-orch/")
    }

    /// Tools whose call puts a session in waiting.
    #[must_use]
    pub fn waiting_tools(&self) -> Vec<String> {
        self.waiting_tools.clone().unwrap_or_else(|| {
            ["AskUserQuestion", "ExitPlanMode"]
                .map(str::to_owned)
                .to_vec()
        })
    }

    /// Notification types that put a session in waiting.
    #[must_use]
    pub fn waiting_notifications(&self) -> Vec<String> {
        self.waiting_notifications.clone().unwrap_or_else(|| {
            [
                "permission_prompt",
                "elicitation_dialog",
                "elicitation_url_dialog",
                "agent_needs_input",
            ]
            .map(str::to_owned)
            .to_vec()
        })
    }

    /// The ledger's timestamp granularity in milliseconds.
    #[must_use]
    pub fn ledger_second_ms(&self) -> i64 {
        self.ledger_second_ms.unwrap_or(1_000)
    }

    /// Refused unknown sessions remembered.
    #[must_use]
    pub fn max_refused_unknown(&self) -> usize {
        self.max_refused_unknown.unwrap_or(32)
    }

    /// Characters of an identifier.
    #[must_use]
    pub fn max_id_len(&self) -> usize {
        self.max_id_len.unwrap_or(62)
    }
}

/// The start-time keys; the install engine restarts the daemon when one changes. Each field is the key of the same name.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Startup {
    /// `staging_target`.
    pub staging_target: Option<String>,
    /// `peek_addr`.
    pub peek_addr: Option<SocketAddr>,
    /// `peek_retry_ms`.
    pub peek_retry_ms: Option<u64>,
    /// `workers`.
    pub workers: Option<usize>,
    /// `shutdown_timeout_ms`.
    pub shutdown_timeout_ms: Option<u64>,
    /// `inbox_capacity`.
    pub inbox_capacity: Option<usize>,
    /// `worker_branch_prefix`.
    pub worker_branch_prefix: Option<String>,
    /// `hook_max_body`.
    pub hook_max_body: Option<usize>,
    /// `hook_max_subject`.
    pub hook_max_subject: Option<usize>,
    /// `hook_max_detail`.
    pub hook_max_detail: Option<usize>,
    /// `claim_poll_ms`.
    pub claim_poll_ms: Option<u64>,
    /// `claim_polls`.
    pub claim_polls: Option<u32>,
    /// `client_timeout_ms`.
    pub client_timeout_ms: Option<u64>,
    /// `peek_healthz_timeout_ms`.
    pub peek_healthz_timeout_ms: Option<u64>,
    /// `peek_call_timeout_ms`.
    pub peek_call_timeout_ms: Option<u64>,
    /// `socket_write_timeout_ms`.
    pub socket_write_timeout_ms: Option<u64>,
    /// `scan_settle_ms`.
    pub scan_settle_ms: Option<u64>,
    /// `ledger_tail_window`.
    pub ledger_tail_window: Option<u64>,
    /// `transcript_max_lines`.
    pub transcript_max_lines: Option<usize>,
    /// `exits_batch`.
    pub exits_batch: Option<usize>,
    /// `exits_poll_ms`.
    pub exits_poll_ms: Option<u64>,
}

impl Startup {
    /// `None` when unset or empty: no staging target (D11 — the done skill refuses `branch:`).
    #[must_use]
    pub fn staging_target(&self) -> Option<&str> {
        self.staging_target
            .as_deref()
            .filter(|target| !target.is_empty())
    }

    /// The central peek's loopback address.
    #[must_use]
    pub fn peek_addr(&self) -> SocketAddr {
        self.peek_addr.unwrap_or_else(|| {
            DEFAULT_PEEK_ADDR
                .parse()
                .unwrap_or_else(|_| unreachable!("DEFAULT_PEEK_ADDR parses"))
        })
    }

    /// How long a degraded peek round waits before it is repeated.
    #[must_use]
    pub fn peek_retry(&self) -> Duration {
        Duration::from_millis(self.peek_retry_ms.unwrap_or(30_000))
    }

    /// The runtime's worker threads.
    #[must_use]
    pub fn workers(&self) -> usize {
        self.workers.unwrap_or(2)
    }

    /// How long shutdown waits for the runtime.
    #[must_use]
    pub fn shutdown_timeout(&self) -> Duration {
        Duration::from_millis(self.shutdown_timeout_ms.unwrap_or(500))
    }

    /// The engine inbox's capacity.
    #[must_use]
    pub fn inbox_capacity(&self) -> usize {
        self.inbox_capacity.unwrap_or(16_384)
    }

    /// The prefix of worker branches.
    #[must_use]
    pub fn worker_branch_prefix(&self) -> &str {
        self.worker_branch_prefix.as_deref().unwrap_or("claude/")
    }

    /// Bytes of a hook body.
    #[must_use]
    pub fn hook_max_body(&self) -> usize {
        self.hook_max_body.unwrap_or(64 * 1024 * 1024)
    }

    /// Characters of a hook subject.
    #[must_use]
    pub fn hook_max_subject(&self) -> usize {
        self.hook_max_subject.unwrap_or(256)
    }

    /// Characters of a hook detail.
    #[must_use]
    pub fn hook_max_detail(&self) -> usize {
        self.hook_max_detail.unwrap_or(2048)
    }

    /// The interval of one claim poll.
    #[must_use]
    pub fn claim_poll(&self) -> Duration {
        Duration::from_millis(self.claim_poll_ms.unwrap_or(10))
    }

    /// Claim polls before giving up.
    #[must_use]
    pub fn claim_polls(&self) -> u32 {
        self.claim_polls.unwrap_or(200)
    }

    /// The CLI client's request timeout.
    #[must_use]
    pub fn client_timeout(&self) -> Duration {
        Duration::from_millis(self.client_timeout_ms.unwrap_or(5_000))
    }

    /// The peek health check's timeout.
    #[must_use]
    pub fn peek_healthz_timeout(&self) -> Duration {
        Duration::from_millis(self.peek_healthz_timeout_ms.unwrap_or(5_000))
    }

    /// The peek call's timeout.
    #[must_use]
    pub fn peek_call_timeout(&self) -> Duration {
        Duration::from_millis(self.peek_call_timeout_ms.unwrap_or(10_000))
    }

    /// The messaging socket's write timeout.
    #[must_use]
    pub fn socket_write_timeout(&self) -> Duration {
        Duration::from_millis(self.socket_write_timeout_ms.unwrap_or(5_000))
    }

    /// How long an agentwt scan waits for writes to settle.
    #[must_use]
    pub fn scan_settle(&self) -> Duration {
        Duration::from_millis(self.scan_settle_ms.unwrap_or(250))
    }

    /// Bytes of the denial ledger read at start.
    #[must_use]
    pub fn ledger_tail_window(&self) -> u64 {
        self.ledger_tail_window.unwrap_or(1024 * 1024)
    }

    /// Transcript lines read per pass.
    #[must_use]
    pub fn transcript_max_lines(&self) -> usize {
        self.transcript_max_lines.unwrap_or(64)
    }

    /// Process exits forwarded per batch.
    #[must_use]
    pub fn exits_batch(&self) -> usize {
        self.exits_batch.unwrap_or(64)
    }

    /// Interval of the polling process-exit watch off darwin.
    #[must_use]
    pub fn exits_poll(&self) -> Duration {
        Duration::from_millis(self.exits_poll_ms.unwrap_or(1_000))
    }
}

/// The branch-name prefixes whose branches are collision lanes (merge strategy D10). An unset key
/// stays `None` and the default lives in the accessor; a set list is used as it is, so an empty list means no lanes.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LanePrefixes {
    /// `collision_prefixes`.
    pub prefixes: Option<Vec<String>>,
}

impl LanePrefixes {
    /// The lanes of a config that does not set the key: the session branches.
    pub const DEFAULT: [&'static str; 1] = ["claude/"];

    /// A branch with this name is a collision lane.
    #[must_use]
    pub fn admits(&self, branch: &str) -> bool {
        match &self.prefixes {
            Some(prefixes) => prefixes
                .iter()
                .any(|prefix| branch.starts_with(prefix.as_str())),
            None => Self::DEFAULT
                .iter()
                .any(|prefix| branch.starts_with(prefix)),
        }
    }
}

/// Exempt paths and version keys (concept D12).
#[derive(Debug, Clone)]
pub struct Exemptions {
    set: GlobSet,
    patterns: Vec<String>,
    version_keys: Vec<String>,
}

impl Default for Exemptions {
    fn default() -> Self {
        Exemptions {
            set: GlobSet::empty(),
            patterns: Vec::new(),
            version_keys: Vec::new(),
        }
    }
}

impl PartialEq for Exemptions {
    fn eq(&self, other: &Self) -> bool {
        self.patterns == other.patterns && self.version_keys == other.version_keys
    }
}

impl Exemptions {
    /// Builds the set; the first pattern that does not compile is the error.
    pub fn new(patterns: &[String], version_keys: Vec<String>) -> crate::Result<Exemptions> {
        let set = glob_set("exempt_paths", patterns, true)?;
        Ok(Exemptions {
            set,
            patterns: patterns.to_vec(),
            version_keys,
        })
    }

    /// The repo-relative path never collides.
    #[must_use]
    pub fn path(&self, rel: &str) -> bool {
        self.set.is_match(rel)
    }

    /// Every non-blank line assigns a version key (`version: 7.1.0`, `"version": "1.2",`, `version = "0.1"`);
    /// `false` when there is no such line.
    #[must_use]
    pub fn version_only(&self, lines: &[&str]) -> bool {
        let mut any = false;
        for line in lines.iter().filter(|line| !line.trim().is_empty()) {
            if !self.is_version_line(line) {
                return false;
            }
            any = true;
        }
        any
    }

    fn is_version_line(&self, line: &str) -> bool {
        let text = line.trim_start().trim_start_matches('"');
        self.version_keys.iter().any(|key| {
            text.strip_prefix(key.as_str()).is_some_and(|rest| {
                let rest = rest.trim_start_matches('"').trim_start();
                rest.starts_with(':') || rest.starts_with('=')
            })
        })
    }
}

/// Glob patterns over model ids and `Agent`-call aliases (`*fable*` matches `fable`, `claude-fable-5-1` and
/// `anthropic/claude-fable-5-1`); empty = nothing forbidden.
#[derive(Debug, Clone)]
pub struct ForbiddenModels {
    set: GlobSet,
    patterns: Vec<String>,
}

impl Default for ForbiddenModels {
    fn default() -> Self {
        ForbiddenModels {
            set: GlobSet::empty(),
            patterns: Vec::new(),
        }
    }
}

impl PartialEq for ForbiddenModels {
    fn eq(&self, other: &Self) -> bool {
        self.patterns == other.patterns
    }
}

impl ForbiddenModels {
    /// Builds the set; the first pattern that does not compile is the error.
    pub fn new(patterns: &[String]) -> crate::Result<ForbiddenModels> {
        let set = glob_set("forbidden_subagent_models", patterns, false)?;
        Ok(ForbiddenModels {
            set,
            patterns: patterns.to_vec(),
        })
    }

    /// The model is forbidden.
    #[must_use]
    pub fn matches(&self, model: &str) -> bool {
        self.set.is_match(model)
    }
}

/// Compiles `patterns` into one set; the first bad pattern is the error.
fn glob_set(key: &str, patterns: &[String], literal_separator: bool) -> crate::Result<GlobSet> {
    let mut builder = GlobSetBuilder::new();
    for pattern in patterns {
        let glob = GlobBuilder::new(pattern)
            .literal_separator(literal_separator)
            .build()
            .map_err(|err| crate::Error::Config(format!("{key}: {pattern}: {err}")))?;
        builder.add(glob);
    }
    builder
        .build()
        .map_err(|err| crate::Error::Config(format!("{key}: {err}")))
}

/// Parses the config body strictly; every refusal names the key.
pub fn parse(bytes: &[u8]) -> crate::Result<FileConfig> {
    let raw: RawConfig =
        serde_json::from_slice(bytes).map_err(|err| crate::Error::Config(err.to_string()))?;
    let exemptions = Exemptions::new(
        raw.exempt_paths.as_deref().unwrap_or(&[]),
        raw.version_keys
            .unwrap_or_else(|| vec!["version".to_owned()]),
    )?;
    let forbidden_models =
        ForbiddenModels::new(raw.forbidden_subagent_models.as_deref().unwrap_or(&[]))?;
    let live = Live {
        exemptions,
        lane_prefixes: LanePrefixes {
            prefixes: raw.collision_prefixes,
        },
        forbidden_models,
        texts: Texts {
            stop_reason: raw.runtime_stop_reason,
            kill_reason: raw.kill_reason,
            subscriber_deny_reason: raw.subscriber_deny_reason,
            attack_deny_reason: raw.attack_deny_reason,
            attack_stop_reason: raw.attack_stop_reason,
            attack_inherited_reason: raw.attack_inherited_reason,
            attack_substituted_reason: raw.attack_substituted_reason,
            member_deny_reason: raw.member_deny_reason,
            member_role_text: raw.member_role_text,
        },
        caps: Caps {
            kill_at: raw.kill_at,
            max_ended_runs: raw.max_ended_runs,
            runtime_digest_lines: raw.runtime_digest_lines,
            guarded_tools: raw.guarded_tools,
            max_field: raw.max_field,
            digest_lines: raw.digest_lines,
            digest_line_paths: raw.digest_line_paths,
            max_terminal: raw.max_terminal,
            merge_branch_prefix: raw.merge_branch_prefix,
            waiting_tools: raw.waiting_tools,
            waiting_notifications: raw.waiting_notifications,
            ledger_second_ms: raw.ledger_second_ms,
            max_refused_unknown: raw.max_refused_unknown,
            max_id_len: raw.max_id_len,
        },
    };
    let startup = Startup {
        staging_target: raw.staging_target,
        peek_addr: raw.peek_addr,
        peek_retry_ms: raw.peek_retry_ms,
        workers: raw.workers,
        shutdown_timeout_ms: raw.shutdown_timeout_ms,
        inbox_capacity: raw.inbox_capacity,
        worker_branch_prefix: raw.worker_branch_prefix,
        hook_max_body: raw.hook_max_body,
        hook_max_subject: raw.hook_max_subject,
        hook_max_detail: raw.hook_max_detail,
        claim_poll_ms: raw.claim_poll_ms,
        claim_polls: raw.claim_polls,
        client_timeout_ms: raw.client_timeout_ms,
        peek_healthz_timeout_ms: raw.peek_healthz_timeout_ms,
        peek_call_timeout_ms: raw.peek_call_timeout_ms,
        socket_write_timeout_ms: raw.socket_write_timeout_ms,
        scan_settle_ms: raw.scan_settle_ms,
        ledger_tail_window: raw.ledger_tail_window,
        transcript_max_lines: raw.transcript_max_lines,
        exits_batch: raw.exits_batch,
        exits_poll_ms: raw.exits_poll_ms,
    };
    Ok(FileConfig { live, startup })
}

/// Every accessor's value under its key in the consumer form (durations as `<key>_ms`, lists as arrays), the mirror
/// of the Go `RenderDefaults`; `guards.rs` compares it with the catalog fixture.
#[cfg(test)]
#[must_use]
pub fn render_defaults(config: &FileConfig) -> serde_json::Value {
    use serde_json::json;

    fn ms(duration: Duration) -> u64 {
        u64::try_from(duration.as_millis()).unwrap_or(u64::MAX)
    }
    let live = &config.live;
    let caps = &live.caps;
    let texts = &live.texts;
    let startup = &config.startup;
    let collision_prefixes: Vec<String> = match &live.lane_prefixes.prefixes {
        Some(prefixes) => prefixes.clone(),
        None => LanePrefixes::DEFAULT.map(str::to_owned).to_vec(),
    };
    let pairs = [
        ("attack_deny_reason", json!(texts.attack_deny_reason())),
        (
            "attack_inherited_reason",
            json!(texts.attack_inherited_reason()),
        ),
        ("attack_stop_reason", json!(texts.attack_stop_reason())),
        (
            "attack_substituted_reason",
            json!(texts.attack_substituted_reason()),
        ),
        ("claim_poll_ms", json!(ms(startup.claim_poll()))),
        ("claim_polls", json!(startup.claim_polls())),
        ("client_timeout_ms", json!(ms(startup.client_timeout()))),
        ("collision_prefixes", json!(collision_prefixes)),
        ("digest_line_paths", json!(caps.digest_line_paths())),
        ("digest_lines", json!(caps.digest_lines())),
        ("exempt_paths", json!(live.exemptions.patterns)),
        ("exits_batch", json!(startup.exits_batch())),
        ("exits_poll_ms", json!(startup.exits_poll().as_millis())),
        (
            "forbidden_subagent_models",
            json!(live.forbidden_models.patterns),
        ),
        ("guarded_tools", json!(caps.guarded_tools())),
        ("hook_max_body", json!(startup.hook_max_body())),
        ("hook_max_detail", json!(startup.hook_max_detail())),
        ("hook_max_subject", json!(startup.hook_max_subject())),
        ("inbox_capacity", json!(startup.inbox_capacity())),
        ("kill_at", json!(caps.kill_at())),
        ("kill_reason", json!(texts.kill_reason())),
        ("ledger_second_ms", json!(caps.ledger_second_ms())),
        ("ledger_tail_window", json!(startup.ledger_tail_window())),
        ("max_ended_runs", json!(caps.max_ended_runs())),
        ("max_field", json!(caps.max_field())),
        ("max_id_len", json!(caps.max_id_len())),
        ("max_refused_unknown", json!(caps.max_refused_unknown())),
        ("max_terminal", json!(caps.max_terminal())),
        ("member_deny_reason", json!(texts.member_deny_reason())),
        ("member_role_text", json!(texts.member_role_text())),
        ("merge_branch_prefix", json!(caps.merge_branch_prefix())),
        (
            "peek_call_timeout_ms",
            json!(ms(startup.peek_call_timeout())),
        ),
        (
            "peek_healthz_timeout_ms",
            json!(ms(startup.peek_healthz_timeout())),
        ),
        ("peek_retry_ms", json!(ms(startup.peek_retry()))),
        ("runtime_digest_lines", json!(caps.runtime_digest_lines())),
        ("runtime_stop_reason", json!(texts.stop_reason())),
        ("scan_settle_ms", json!(ms(startup.scan_settle()))),
        ("shutdown_timeout_ms", json!(ms(startup.shutdown_timeout()))),
        (
            "socket_write_timeout_ms",
            json!(ms(startup.socket_write_timeout())),
        ),
        (
            "staging_target",
            json!(startup.staging_target().unwrap_or("")),
        ),
        (
            "subscriber_deny_reason",
            json!(texts.subscriber_deny_reason()),
        ),
        (
            "transcript_max_lines",
            json!(startup.transcript_max_lines()),
        ),
        ("version_keys", json!(live.exemptions.version_keys)),
        ("waiting_notifications", json!(caps.waiting_notifications())),
        ("waiting_tools", json!(caps.waiting_tools())),
        (
            "worker_branch_prefix",
            json!(startup.worker_branch_prefix()),
        ),
        ("workers", json!(startup.workers())),
    ];
    serde_json::Value::Object(
        pairs
            .into_iter()
            .map(|(key, value)| (key.to_owned(), value))
            .collect(),
    )
}

#[derive(Debug, Deserialize)]
struct RegistryFile {
    #[serde(default)]
    repos: Vec<RegisteredRepo>,
}

/// One entry of the config server's repo registry (`label` and other fields are ignored).
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct RegisteredRepo {
    /// Registry name.
    pub name: String,
    /// Main-checkout path.
    pub path: PathBuf,
}

/// Parses `repos.json`.
pub fn parse_repos(bytes: &[u8]) -> crate::Result<Vec<RegisteredRepo>> {
    let file: RegistryFile = serde_json::from_slice(bytes)?;
    Ok(file.repos)
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    fn config_err(body: &[u8]) -> String {
        match parse(body) {
            Err(crate::Error::Config(text)) => text,
            other => panic!("want Error::Config, got {other:?}"),
        }
    }

    #[test]
    fn empty_yields_defaults() {
        let config = parse(b"{}").expect("parse");
        assert_eq!(config.live.texts, Texts::default());
        assert_eq!(config.live.caps, Caps::default());
        assert_eq!(config.startup, Startup::default());
        assert!(!config.live.exemptions.path("plans/a/design/raw.md"));
        assert!(!config.live.forbidden_models.matches("claude-fable-5-1"));
        let caps = &config.live.caps;
        assert_eq!(caps.kill_at(), 8);
        assert_eq!(caps.max_ended_runs(), 64);
        assert_eq!(caps.max_terminal(), 1_000);
        assert_eq!(caps.merge_branch_prefix(), "claude-orch/");
        assert_eq!(caps.waiting_tools(), ["AskUserQuestion", "ExitPlanMode"]);
        assert_eq!(caps.max_id_len(), 62);
        let startup = &config.startup;
        assert_eq!(startup.staging_target(), None);
        assert_eq!(startup.peek_addr().to_string(), DEFAULT_PEEK_ADDR);
        assert_eq!(startup.peek_retry(), Duration::from_secs(30));
        assert_eq!(startup.workers(), 2);
        assert_eq!(startup.inbox_capacity(), 16_384);
        assert_eq!(startup.worker_branch_prefix(), "claude/");
        assert_eq!(startup.hook_max_body(), 67_108_864);
        assert_eq!(startup.ledger_tail_window(), 1_048_576);
        assert!(config.live.texts.stop_reason().contains("{finding}"));
        assert!(config.live.texts.kill_reason().contains("{kill_at}"));
    }

    #[test]
    fn set_keys_override_defaults() {
        let config = parse(
            br#"{"runtime_stop_reason": "s {finding}", "kill_at": 3, "staging_target": "develop",
                 "peek_addr": "127.0.0.1:9", "scan_settle_ms": 7}"#,
        )
        .expect("parse");
        assert_eq!(config.live.texts.stop_reason(), "s {finding}");
        assert_eq!(config.live.caps.kill_at(), 3);
        assert_eq!(config.startup.staging_target(), Some("develop"));
        assert_eq!(config.startup.peek_addr().to_string(), "127.0.0.1:9");
        assert_eq!(config.startup.scan_settle(), Duration::from_millis(7));
    }

    #[test]
    fn unknown_key_err() {
        let text = config_err(br#"{"exempt_paths": [], "train_bases": {"/r1": ["develop"]}}"#);
        assert!(text.contains("train_bases"), "{text}");
    }

    #[test]
    fn wrong_type_err() {
        let cases: [(&str, &[u8]); 3] = [
            ("list as string", br#"{"collision_prefixes": "claude/"}"#),
            ("int as string", br#"{"kill_at": "x"}"#),
            ("bad address", br#"{"peek_addr": "nowhere"}"#),
        ];
        for (label, body) in cases {
            assert!(
                matches!(parse(body), Err(crate::Error::Config(_))),
                "{label}"
            );
        }
    }

    #[test]
    fn bad_glob_err() {
        let cases: [(&str, &[u8], &str); 2] = [
            (
                "exempt",
                br#"{"exempt_paths": ["[", "plans/**"]}"#,
                "exempt_paths: [:",
            ),
            (
                "forbidden",
                br#"{"forbidden_subagent_models": ["["]}"#,
                "forbidden_subagent_models: [:",
            ),
        ];
        for (label, body, prefix) in cases {
            let text = config_err(body);
            assert!(text.starts_with(prefix), "{label}: {text}");
        }
    }

    #[test]
    fn staging_empty_is_none() {
        let config = parse(br#"{"staging_target": ""}"#).expect("parse");
        assert_eq!(config.startup.staging_target(), None);
    }

    #[test]
    fn version_keys_default() {
        let config = parse(b"{}").expect("parse");
        assert!(config.live.exemptions.version_only(&["version: 1.2.0"]));
        let custom = parse(br#"{"version_keys": ["release"]}"#).expect("parse");
        assert!(!custom.live.exemptions.version_only(&["version: 1.2.0"]));
        assert!(custom.live.exemptions.version_only(&["release: 3"]));
    }

    #[test]
    fn forbidden_models() {
        let config = parse(br#"{"forbidden_subagent_models": ["*fable*"]}"#).expect("parse");
        let cases = [
            ("agent-call-alias", "fable", true),
            ("model-id", "claude-fable-5-1", true),
            ("older-model-id", "claude-fable-5", true),
            ("provider-prefixed", "anthropic/claude-fable-5-1", true),
            ("opus", "claude-opus-5-5", false),
            ("haiku-alias", "haiku", false),
        ];
        for (label, model, expected) in cases {
            assert_eq!(
                config.live.forbidden_models.matches(model),
                expected,
                "{label}"
            );
        }
    }

    #[test]
    fn exempt_paths() {
        let config = parse(
            br#"{"exempt_paths": ["plans/**", "**/changelog.json", "**/CHANGELOG", "**/CHANGELOG.md"]}"#,
        )
        .expect("parse");
        let cases = [
            ("plan-file", "plans/x/design/raw.md", true),
            ("nested-changelog", "skills/a/changelog.json", true),
            ("root-changelog", "changelog.json", true),
            ("changelog-named-source", "src/changelog.rs", false),
            ("changelog-md", "CHANGELOG.md", true),
        ];
        for (label, path, expected) in cases {
            assert_eq!(config.live.exemptions.path(path), expected, "{label}");
        }
    }

    #[test]
    fn version_only() {
        let exemptions = Exemptions::new(&[], vec!["version".to_owned()]).expect("exemptions");
        let cases: [(&str, &[&str], bool); 6] = [
            ("yaml", &["version: 1.2.0"], true),
            ("json", &[r#""version": "1.2","#], true),
            ("toml", &[r#"version = "0.1""#], true),
            ("other-key", &["versions: 3"], false),
            (
                "version-plus-code",
                &["version: 1.2.0", "fn main() {}"],
                false,
            ),
            ("only-blank", &["", "  "], false),
        ];
        for (label, lines, expected) in cases {
            assert_eq!(exemptions.version_only(lines), expected, "{label}");
        }
    }

    #[test]
    fn parse_repos_reads_name_and_path() {
        let repos = parse_repos(br#"{"repos": [{"label": "L", "name": "cc", "path": "/r/cc"}]}"#)
            .expect("parse");
        let want = vec![RegisteredRepo {
            name: "cc".to_owned(),
            path: PathBuf::from("/r/cc"),
        }];
        assert_eq!(repos, want);
        assert_eq!(
            parse_repos(b"{}").expect("parse"),
            Vec::<RegisteredRepo>::new()
        );
    }

    #[test]
    fn collision_prefixes() {
        let branches = [
            "claude/agent-a1",
            "claude-orch/T1-0001",
            "claude-routines/nightly",
            "main",
        ];
        let cases: [(&str, &[u8], [bool; 4]); 3] = [
            ("key absent", b"{}", [true, false, false, false]),
            (
                "explicit two prefixes",
                br#"{"collision_prefixes": ["claude/", "claude-orch/"]}"#,
                [true, true, false, false],
            ),
            (
                "empty list admits nothing",
                br#"{"collision_prefixes": []}"#,
                [false, false, false, false],
            ),
        ];
        for (label, body, want) in cases {
            let config = parse(body).expect("parse");
            let got = branches.map(|branch| config.live.lane_prefixes.admits(branch));
            assert_eq!(got, want, "{label}");
        }
    }
}
