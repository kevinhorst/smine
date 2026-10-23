//! protocol.rs — the protocol facts: names both sides of an interface parse, never configuration.
//! The literal guard (`guards.rs`) exempts this file and `config.rs` alone.

use crate::plan::{Lang, Shape};

/// Schema version of the status document (`schema/status-v1.json`).
pub const STATUS_SCHEMA: u32 = 1;
/// Schema version of the runtime document (`schema/runtime-v1.json`).
pub const RUNTIME_SCHEMA: u32 = 1;
/// Format version of a fleet archive file.
pub const ARCHIVE_VERSION: u32 = 1;
/// Line version of the action ledger.
pub const ACTION_LEDGER_VERSION: u32 = 1;
/// The shortest fence opener of a plan file's fenced block (`CommonMark`: three backticks or tildes).
pub const FENCE_MIN_WIDTH: usize = 3;
/// The symbol signatures a plan file's diff and header lines are read for, per language and capture shape — the
/// syntax of the languages the plans name.
pub(crate) const PLAN_SIGNATURES: [(Lang, Shape, &str); 9] = [
    (
        Lang::Rust,
        Shape::Name,
        r#"^\s*(?:pub(?:\([^)]*\))?\s+)?(?:(?:const|async|unsafe)\s+)*(?:extern\s+"[^"]*"\s+)?fn\s+([A-Za-z_]\w*)"#,
    ),
    (
        Lang::Rust,
        Shape::Name,
        r"^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|union|mod)\s+([A-Za-z_]\w*)",
    ),
    (
        Lang::Rust,
        Shape::Impl,
        r"^\s*impl(?:<[^>]*>)?\s+(?:[\w:<>, &']+\s+for\s+)?([A-Za-z_][\w:]*)",
    ),
    (
        Lang::Go,
        Shape::Method,
        r"^func\s+\(\s*(?:\w+\s+)?\*?\s*(\w+)(?:\[[^\]]*\])?\s*\)\s*(\w+)",
    ),
    (Lang::Go, Shape::Name, r"^func\s+(\w+)"),
    (Lang::Go, Shape::Name, r"^type\s+(\w+)\s"),
    (
        Lang::Python,
        Shape::Name,
        r"^\s*(?:async\s+)?(?:def|class)\s+(\w+)",
    ),
    (
        Lang::Shell,
        Shape::Name,
        r"^\s*(?:function\s+)?([A-Za-z_][\w-]*)\s*\(\)\s*\{",
    ),
    (
        Lang::Js,
        Shape::Name,
        r"^\s*(?:export\s+)?(?:async\s+)?function\s*\*?\s*(\w+)",
    ),
];
/// The CLI's exit code when no daemon answers.
pub const EXIT_UNREACHABLE: i32 = 3;
/// The CLI's exit code when the daemon serves another Claude home.
pub const EXIT_FOREIGN_HOME: i32 = 4;
/// The exit code base of a signal: `128 + signal`.
pub const EXIT_SIGNAL_BASE: i32 = 128;
/// The encoding a path must have to enter the status document.
pub const PATH_ENCODING: &str = "UTF-8";
/// The hook URL path Claude Code posts to.
pub const HOOK_PATH: &str = "/hook";
/// The daemon's compiled address; the catalog's `heartbeat.port` default mirrors its port and the fixture test pins it.
pub const DEFAULT_ADDR: &str = "127.0.0.1:6101";
/// The central peek's compiled address; the catalog's `peek.port` default mirrors its port.
pub const DEFAULT_PEEK_ADDR: &str = "127.0.0.1:4242";
/// The `--session` value naming the calling session.
pub const SELF_SESSION: &str = "self";
/// The process env carrying the calling Claude Code session id.
pub const SESSION_ENV: &str = "CLAUDE_CODE_SESSION_ID";
/// The process env carrying the session's messaging socket.
pub const SOCKET_ENV: &str = "CLAUDE_CODE_MESSAGING_SOCKET";
/// The process env carrying the session's messaging token.
pub const TOKEN_ENV: &str = "CLAUDE_CODE_MESSAGING_TOKEN";
/// The user-facing grammar of a fleet id.
pub const FLEET_ID_CHARSET: &str = "[A-Za-z0-9-]+";
/// The user-facing charset of a branch-name target.
pub const BRANCH_CHARSET: &str = "[A-Za-z0-9._/-]";
/// The member-letter alphabet (`A` … `Z`, then `AA`): titles and digests carry the letters.
pub const MEMBER_LETTERS: &[u8; 26] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZ";
/// Name bytes a legacy fleet-run id derives from: every replay of an old ledger must derive the same id.
pub const LEGACY_NAME_BYTES: usize = 10;
/// Letters of the longest member letter `/remove` accepts (`ZZZZ`): the member grammar the CLI and titles share.
pub const MAX_MEMBER_LETTERS: usize = 4;
/// Column widths of the `runtime status` text table: RUN, STATE, SKILL, VERSION, FINDINGS, FIRED.
pub const RUNTIME_TABLE_WIDTHS: [usize; 6] = [6, 10, 24, 9, 8, 6];
/// Column widths of the `status` text table: SESSION, STATE, SINCE.
pub const STATUS_TABLE_WIDTHS: [usize; 3] = [10, 9, 20];
/// Column widths of the `status --fleet <id>` member block: TOKEN, VIA, DELIVERED.
pub const MEMBER_TABLE_WIDTHS: [usize; 3] = [8, 11, 9];
/// Column widths of the `status --fleet` list: FLEET, RUN, TARGET, STATE, MEMBERS.
pub const FLEET_TABLE_WIDTHS: [usize; 5] = [8, 9, 32, 11, 7];
/// The fleet filter selecting every fleet.
pub const EVERY_FLEET: &str = "*";
/// Claude Code's skill tool name.
pub const SKILL_TOOL: &str = "Skill";
/// Claude Code's subagent tool name.
pub const AGENT_TOOL: &str = "Agent";
/// Claude Code's message tool name.
pub const SEND_MESSAGE_TOOL: &str = "SendMessage";
/// The `SessionStart` source of a compaction.
pub const COMPACT_SOURCE: &str = "compact";
/// The session registry's busy status.
pub const BUSY_STATUS: &str = "busy";
/// The session registry's waiting status.
pub const WAITING_STATUS: &str = "waiting";
/// The waiting cause read from the session registry's `waitingFor`.
pub const REGISTRY_WAIT: &str = "registry waitingFor";
/// The end cause of a session whose process exited without a `SessionEnd` hook.
pub const EXIT_WITHOUT_END: &str = "process exited without SessionEnd";
/// The toolpolicy denial ledger under the Claude home.
pub const LEDGER_PATH: &str = "tool-policy/denials.jsonl";
/// The heartbeat dir under the Claude home.
pub const STATE_DIR: &str = "heartbeat";
/// The heartbeat config file in [`STATE_DIR`].
pub const CONFIG_FILE: &str = "config.json";
/// The unix signal numbers a windows console event is reported as (`Error::Interrupted`).
pub const INTERRUPT_SIGNAL: i32 = 2;
/// The unix signal number a windows console close is reported as (`Error::Interrupted`).
pub const TERMINATE_SIGNAL: i32 = 15;
/// The socket archive dir.
pub const ARCHIVE_DIR: &str = "archive";
/// The action ledger file in [`STATE_DIR`].
pub const ACTIONS_FILE: &str = "actions.jsonl";
/// A skill's enforce declaration file, rendered beside a deployed SKILL.md.
pub const ENFORCE_FILE: &str = "enforce.json";
/// The enforce declaration's schema number.
pub const ENFORCE_SCHEMA: u32 = 1;
/// The enforce evidence prefix naming an input pointer instead of a capture.
pub const ARG_PREFIX: &str = "arg:";
/// Claude Code's shell tool, whose response is `{stdout, stderr, …}`: captures read the JSON document `stdout` ends
/// with.
pub const BASH_TOOL: &str = "Bash";
/// The session switch feature name of heartbeat.
pub const FEATURE: &str = "hb";
/// The refusal text of a landing call by a session that does not hold the turn.
pub const NOT_HOLDER: &str = "not the turn holder";
/// The tool-result prefix of an async `Agent` launch.
pub const ASYNC_LAUNCH_ACK: &str = "Async agent launched";
/// peek's MCP tool that answers a session's spawn tree.
pub const PEEK_SESSION_EVENTS_TOOL: &str = "session_events";
/// The session registry dir under the Claude home.
pub const SESSIONS_DIR: &str = "sessions";
/// The transcript dir under the Claude home.
pub const PROJECTS_DIR: &str = "projects";
/// The container sessions' registry dir under the Claude home: the routine runner writes it for container cells; the
/// docker client's host pid stands for a session whose harness registry file stays inside its container.
pub const CONTAINER_SESSIONS_DIR: &str = "smine/container-sessions";
