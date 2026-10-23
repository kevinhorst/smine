//! Runtime enforcement: a skill's deployed `enforce.json` and the four check kinds over its anchor log. The hook edge
//! reads and compiles the declaration when a hook opens a run ([`EnforceDoc::parse`]); everything else here is pure —
//! no file, no clock — and runs inside `apply` over the run's ordered anchor log.
//!
//! Anchors are named calls: a tool regex, input patterns that must match, input patterns that must not, and whose
//! calls count (root, subagent or any). A successful `PostToolUse` of an anchored call appends an [`Entry`] to the
//! run's log and records the anchor's captures on that call's log entry, read from the response (a Bash response's
//! `stdout`: the JSON document it ends with, log lines ahead of it skipped); a capture the response lacks is absent.
//!
//! Checks (D5): `provenance` compares a value of a call's input with a capture of the last `from` call — a `from` call
//! without the value is a breach naming it absent; `requires`
//! wants an `after` anchor before a call, after the last `since` anchor when given; `phase` denies a call between a
//! `from` success and the next `until` success; `stop` blocks the root's Stop while an `after` anchor has no `require`
//! anchor after it.
//!
//! Every check whose guard matches is judged on each hook ([`Evaluation`]): a provenance or requires check when its
//! `on` anchor matches the call, a phase check when its `deny` anchor does, a stop check on a judged root Stop once its
//! `after` anchor succeeded. The checks that held are passes; the first that broke, in id order, is the failure.

use std::collections::BTreeMap;
use std::fmt;

use regex::Regex;
use serde::Deserialize;
use serde_json::Value;

use crate::protocol::{ARG_PREFIX, BASH_TOOL, ENFORCE_SCHEMA};

/// Whose calls an anchor matches.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AnchorAgent {
    /// The root session only (the default).
    Root,
    /// Subagents only.
    Subagent,
    /// Anyone.
    Any,
}

/// One input condition: the top-level field's string value (or compact JSON) against a pattern.
#[derive(Debug, Clone)]
pub struct InputMatch {
    /// The `tool_input` field.
    pub field: String,
    /// The compiled pattern.
    pub pattern: Regex,
}

/// One named call.
#[derive(Debug, Clone)]
pub struct Anchor {
    /// The tool name pattern.
    pub tool: Regex,
    /// Conditions that must all match.
    pub input: Vec<InputMatch>,
    /// Conditions none of which may match.
    pub exclude: Vec<InputMatch>,
    /// Whose calls count.
    pub agent: AnchorAgent,
    /// `(name, pointer)`: response values recorded on success.
    pub capture: Vec<(String, String)>,
}

/// What a check compares.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CheckKind {
    /// The input value at `arg` of an `on` call equals capture `capture` of the last `from` anchor.
    Provenance {
        /// The guarded call.
        on: String,
        /// The input pointer.
        arg: String,
        /// The producing anchor.
        from: String,
        /// The capture name.
        capture: String,
    },
    /// An `on` call needs an `after` anchor, after the last `since` anchor when given.
    Requires {
        /// The guarded call.
        on: String,
        /// The required anchor.
        after: String,
        /// The resetting anchor.
        since: Option<String>,
    },
    /// A `deny` call is refused between a `from` success and the next `until` success.
    Phase {
        /// The denied call.
        deny: String,
        /// Opens the phase.
        from: String,
        /// Closes it.
        until: String,
    },
    /// The root's Stop is blocked while `after` has no `require` after it.
    Stop {
        /// The anchor that must follow.
        require: String,
        /// The anchor that makes it due.
        after: String,
    },
}

/// One check with its fixed reason and its recovery template.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Check {
    /// What it compares.
    pub kind: CheckKind,
    /// The deny or block reason.
    pub reason: String,
    /// The template a `check_failed` finding frees for the runtime agent.
    pub recovery: String,
}

/// One judgment point for the runtime agent.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Judgment {
    /// The anchor whose success raises it.
    pub anchor: String,
    /// Capture names or `arg:<pointer>` of the anchor call's input.
    pub evidence: Vec<String>,
    /// What the runtime agent decides.
    pub question: String,
    /// The template it may send.
    pub send: String,
}

/// A skill's compiled enforce declaration.
#[derive(Debug, Clone)]
pub struct EnforceDoc {
    /// The skill leaf.
    pub skill: String,
    /// The deployed version.
    pub version: String,
    /// Anchors by id.
    pub anchors: BTreeMap<String, Anchor>,
    /// Checks by id.
    pub checks: BTreeMap<String, Check>,
    /// Judgment points by id.
    pub judgment: BTreeMap<String, Judgment>,
}

impl PartialEq for EnforceDoc {
    /// A declaration is identified by its skill and version (compiled patterns have no equality).
    fn eq(&self, other: &EnforceDoc) -> bool {
        self.skill == other.skill && self.version == other.version
    }
}

/// Why a declaration was not loaded.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("enforce.json: {0}")]
pub struct EnforceError(pub String);

#[derive(Deserialize)]
struct RawDoc {
    schema: u32,
    skill: String,
    version: String,
    #[serde(default)]
    anchors: BTreeMap<String, RawAnchor>,
    #[serde(default)]
    checks: BTreeMap<String, RawCheck>,
    #[serde(default)]
    judgment: BTreeMap<String, RawJudgment>,
}

#[derive(Deserialize)]
struct RawAnchor {
    tool: String,
    #[serde(default)]
    input: Vec<RawInput>,
    #[serde(default)]
    exclude: Vec<RawInput>,
    #[serde(default)]
    agent: Option<String>,
    #[serde(default)]
    capture: Vec<RawCapture>,
}

#[derive(Deserialize)]
struct RawInput {
    field: String,
    pattern: String,
}

#[derive(Deserialize)]
struct RawCapture {
    name: String,
    pointer: String,
}

#[derive(Deserialize)]
struct RawCheck {
    kind: String,
    reason: String,
    recovery: String,
    #[serde(default)]
    on: String,
    #[serde(default)]
    arg: String,
    #[serde(default)]
    from: String,
    #[serde(default)]
    capture: String,
    #[serde(default)]
    after: String,
    #[serde(default)]
    since: Option<String>,
    #[serde(default)]
    deny: String,
    #[serde(default)]
    until: String,
    #[serde(default)]
    require: String,
}

#[derive(Deserialize)]
struct RawJudgment {
    anchor: String,
    evidence: Vec<String>,
    question: String,
    send: String,
}

impl EnforceDoc {
    /// Parses and compiles a rendered `enforce.json`.
    pub fn parse(bytes: &[u8]) -> Result<EnforceDoc, EnforceError> {
        let raw: RawDoc =
            serde_json::from_slice(bytes).map_err(|err| EnforceError(err.to_string()))?;
        if raw.schema != ENFORCE_SCHEMA {
            return Err(EnforceError(format!(
                "schema {}, want {ENFORCE_SCHEMA}",
                raw.schema
            )));
        }
        let mut anchors = BTreeMap::new();
        for (id, anchor) in raw.anchors {
            anchors.insert(id.clone(), compile_anchor(&id, anchor)?);
        }
        let mut checks = BTreeMap::new();
        for (id, check) in raw.checks {
            checks.insert(id.clone(), compile_check(&id, check)?);
        }
        let judgment = raw
            .judgment
            .into_iter()
            .map(|(id, point)| {
                let compiled = Judgment {
                    anchor: point.anchor,
                    evidence: point.evidence,
                    question: point.question,
                    send: point.send,
                };
                (id, compiled)
            })
            .collect();
        Ok(EnforceDoc {
            skill: raw.skill,
            version: raw.version,
            anchors,
            checks,
            judgment,
        })
    }
}

fn compile_anchor(id: &str, raw: RawAnchor) -> Result<Anchor, EnforceError> {
    let pattern = |text: &str| {
        Regex::new(text).map_err(|err| EnforceError(format!("anchor {id}: pattern {text}: {err}")))
    };
    let matches = |list: Vec<RawInput>| {
        list.into_iter()
            .map(|input| {
                Ok(InputMatch {
                    pattern: pattern(&input.pattern)?,
                    field: input.field,
                })
            })
            .collect::<Result<Vec<InputMatch>, EnforceError>>()
    };
    let agent = match raw.agent.as_deref() {
        None | Some("root") => AnchorAgent::Root,
        Some("subagent") => AnchorAgent::Subagent,
        Some("any") => AnchorAgent::Any,
        Some(other) => return Err(EnforceError(format!("anchor {id}: unknown agent {other}"))),
    };
    Ok(Anchor {
        tool: pattern(&raw.tool)?,
        input: matches(raw.input)?,
        exclude: matches(raw.exclude)?,
        agent,
        capture: raw
            .capture
            .into_iter()
            .map(|capture| (capture.name, capture.pointer))
            .collect(),
    })
}

fn compile_check(id: &str, raw: RawCheck) -> Result<Check, EnforceError> {
    let kind = match raw.kind.as_str() {
        "provenance" => CheckKind::Provenance {
            on: raw.on,
            arg: raw.arg,
            from: raw.from,
            capture: raw.capture,
        },
        "requires" => CheckKind::Requires {
            on: raw.on,
            after: raw.after,
            since: raw.since,
        },
        "phase" => CheckKind::Phase {
            deny: raw.deny,
            from: raw.from,
            until: raw.until,
        },
        "stop" => CheckKind::Stop {
            require: raw.require,
            after: raw.after,
        },
        other => return Err(EnforceError(format!("check {id}: unknown kind {other}"))),
    };
    Ok(Check {
        kind,
        reason: raw.reason,
        recovery: raw.recovery,
    })
}

impl Anchor {
    /// The call matches: agent, tool, every input condition, no exclusion.
    #[must_use]
    pub fn matches(&self, is_root: bool, tool: &str, input: &Value) -> bool {
        let is_agent = match self.agent {
            AnchorAgent::Root => is_root,
            AnchorAgent::Subagent => !is_root,
            AnchorAgent::Any => true,
        };
        is_agent
            && self.tool.is_match(tool)
            && self.input.iter().all(|condition| condition.matches(input))
            && !self
                .exclude
                .iter()
                .any(|condition| condition.matches(input))
    }
}

impl InputMatch {
    fn matches(&self, input: &Value) -> bool {
        match input.get(&self.field) {
            Some(Value::String(text)) => self.pattern.is_match(text),
            Some(other) => self.pattern.is_match(&other.to_string()),
            None => false,
        }
    }
}

/// A run's ordered anchor log: one entry per successful anchored call with the captures its response yielded.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct AnchorLog {
    /// Successful anchored calls, in apply order.
    pub entries: Vec<Entry>,
}

/// One successful anchored call.
#[derive(Debug, Clone, PartialEq)]
pub struct Entry {
    /// The apply sequence number of the call.
    pub seq: u64,
    /// The anchor id.
    pub anchor: String,
    /// The captures the response yielded, by name; a declared capture the response lacked is absent.
    pub captures: BTreeMap<String, Value>,
}

impl AnchorLog {
    /// The seq of the anchor's last success.
    #[must_use]
    pub fn last(&self, anchor: &str) -> Option<u64> {
        self.last_entry(anchor).map(|entry| entry.seq)
    }

    /// The anchor's last successful call.
    #[must_use]
    pub fn last_entry(&self, anchor: &str) -> Option<&Entry> {
        self.entries
            .iter()
            .rev()
            .find(|entry| entry.anchor == anchor)
    }

    /// The newest value recorded under a capture name, whichever anchor produced it.
    #[must_use]
    pub fn latest_capture(&self, name: &str) -> Option<&Value> {
        self.entries
            .iter()
            .rev()
            .find_map(|entry| entry.captures.get(name))
    }

    /// Records every anchor a successful call matches; returns the matched anchor ids.
    pub fn record(
        &mut self,
        doc: &EnforceDoc,
        seq: u64,
        is_root: bool,
        tool: &str,
        input: &Value,
        response: Option<&str>,
    ) -> Vec<String> {
        let matched: Vec<String> = doc
            .anchors
            .iter()
            .filter(|(_, anchor)| anchor.matches(is_root, tool, input))
            .map(|(id, _)| id.clone())
            .collect();
        if matched.is_empty() {
            return matched;
        }
        let parsed = response.and_then(|text| response_root(tool, text));
        for id in &matched {
            let captures = doc
                .anchors
                .get(id)
                .into_iter()
                .flat_map(|anchor| &anchor.capture)
                .filter_map(|(name, pointer)| {
                    parsed
                        .as_ref()
                        .and_then(|root| resolve(root, pointer))
                        .map(|value| (name.clone(), value))
                })
                .collect();
            self.entries.push(Entry {
                seq,
                anchor: id.clone(),
                captures,
            });
        }
        matched
    }
}

/// The value captures read: the JSON document a Bash response's `stdout` ends with, any other response as is.
fn response_root(tool: &str, text: &str) -> Option<Value> {
    let response: Value = serde_json::from_str(text).ok()?;
    if tool != BASH_TOOL {
        return Some(response);
    }
    let stdout = response.get("stdout")?.as_str()?;
    stdout_document(stdout)
}

/// The capture document of a Bash `stdout`: the suffix from the first line start on which the rest parses as one JSON
/// value (D4). The harness delivers stderr inside `stdout`, ahead of the document a command prints last, so log lines
/// before it are skipped; pure-JSON output parses from its first line, as before.
fn stdout_document(stdout: &str) -> Option<Value> {
    std::iter::once(0)
        .chain(stdout.match_indices('\n').map(|(at, _)| at + 1))
        .filter_map(|start| stdout.get(start..))
        .find_map(|rest| serde_json::from_str::<Value>(rest.trim()).ok())
}

/// RFC 6901 pointer resolution that descends into string values holding JSON (a Workflow's `args` may be a string).
#[must_use]
pub fn resolve(root: &Value, pointer: &str) -> Option<Value> {
    if pointer.is_empty() {
        return Some(root.clone());
    }
    let mut current = root.clone();
    for token in pointer.trim_start_matches('/').split('/') {
        let token = token.replace("~1", "/").replace("~0", "~");
        if let Value::String(text) = &current
            && let Ok(parsed) = serde_json::from_str::<Value>(text)
        {
            current = parsed;
        }
        current = match &current {
            Value::Object(map) => map.get(&token)?.clone(),
            Value::Array(items) => items.get(token.parse::<usize>().ok()?)?.clone(),
            _ => return None,
        };
    }
    Some(current)
}

/// A failed check: its id and its reason.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Failure {
    /// The check id.
    pub check: String,
    /// Its reason.
    pub reason: String,
    /// What was compared, for the finding's evidence.
    pub evidence: Vec<String>,
}

impl fmt::Display for Failure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.check, self.reason)
    }
}

/// One hook's judgment over a declaration: every check whose guard matched, as the ones that held and the first that
/// broke.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Evaluation {
    /// Ids of the guarded checks that held, in id order.
    pub passed: Vec<String>,
    /// The first guarded check that broke, in id order; a later breach on the same hook is not reported.
    pub failure: Option<Failure>,
}

impl Evaluation {
    /// Records one guarded check: no breach is a pass, a breach becomes the failure unless one is already held.
    fn judge(&mut self, id: &str, check: &Check, breach: Option<Vec<String>>) {
        match breach {
            None => self.passed.push(id.to_owned()),
            Some(evidence) if self.failure.is_none() => {
                self.failure = Some(Failure {
                    check: id.to_owned(),
                    reason: check.reason.clone(),
                    evidence,
                });
            }
            Some(_) => {}
        }
    }
}

/// `PreToolUse`: every provenance, requires and phase check whose guarded anchor matches the call, judged.
#[must_use]
pub fn pre_tool(
    doc: &EnforceDoc,
    log: &AnchorLog,
    is_root: bool,
    tool: &str,
    input: &Value,
) -> Evaluation {
    let matches = |anchor: &str| {
        doc.anchors
            .get(anchor)
            .is_some_and(|found| found.matches(is_root, tool, input))
    };
    let mut evaluation = Evaluation::default();
    for (id, check) in &doc.checks {
        let breach = match &check.kind {
            CheckKind::Provenance {
                on,
                arg,
                from,
                capture,
            } if matches(on) => provenance_breach(log, input, arg, from, capture),
            CheckKind::Requires { on, after, since } if matches(on) => {
                let after_seq = log.last(after);
                let since_seq = since.as_deref().and_then(|since| log.last(since));
                // A declared `since` that never happened opens no window: nothing is owed yet.
                let is_met = match (after_seq, since_seq) {
                    (Some(after_seq), Some(since_seq)) => after_seq > since_seq,
                    (Some(_), None) => true,
                    (None, Some(_)) => false,
                    (None, None) => since.is_some(),
                };
                (!is_met).then(|| {
                    vec![format!(
                        "no {after} since the last {}",
                        since.as_deref().unwrap_or("start")
                    )]
                })
            }
            CheckKind::Phase { deny, from, until } if matches(deny) => {
                let is_open = log.last(from).is_some_and(|from_seq| {
                    log.last(until).is_none_or(|until_seq| until_seq < from_seq)
                });
                is_open.then(|| vec![format!("inside {from}..{until}")])
            }
            CheckKind::Provenance { .. }
            | CheckKind::Requires { .. }
            | CheckKind::Phase { .. }
            | CheckKind::Stop { .. } => continue,
        };
        evaluation.judge(id, check, breach);
    }
    evaluation
}

/// The evidence of a provenance breach; `None` when the value matches its capture.
fn provenance_breach(
    log: &AnchorLog,
    input: &Value,
    arg: &str,
    from: &str,
    capture: &str,
) -> Option<Vec<String>> {
    let passed = resolve(input, arg);
    let expected = log
        .last_entry(from)
        .and_then(|entry| entry.captures.get(capture));
    let is_same = match (&passed, expected) {
        (Some(passed), Some(expected)) => same_value(passed, expected),
        (None | Some(_), None) | (None, Some(_)) => false,
    };
    if is_same {
        return None;
    }
    let show = |value: Option<&Value>| value.map_or_else(|| "absent".to_owned(), Value::to_string);
    Some(vec![
        format!("{arg} = {}", show(passed.as_ref())),
        format!("{from}.{capture} = {}", show(expected)),
    ])
}

/// Equal values; a string holding JSON equals the value it encodes.
fn same_value(passed: &Value, expected: &Value) -> bool {
    if passed == expected {
        return true;
    }
    match (passed, expected) {
        (Value::String(text), other) | (other, Value::String(text)) => {
            serde_json::from_str::<Value>(text).is_ok_and(|parsed| parsed == *other)
        }
        _ => false,
    }
}

/// The root's Stop: every stop check whose `after` anchor succeeded, judged — it holds when a `require` followed.
#[must_use]
pub fn on_stop(doc: &EnforceDoc, log: &AnchorLog) -> Evaluation {
    let mut evaluation = Evaluation::default();
    for (id, check) in &doc.checks {
        let (require, after) = match &check.kind {
            CheckKind::Stop { require, after } => (require, after),
            CheckKind::Provenance { .. } | CheckKind::Requires { .. } | CheckKind::Phase { .. } => {
                continue;
            }
        };
        let Some(after_seq) = log.last(after) else {
            continue;
        };
        let is_missing = log
            .last(require)
            .is_none_or(|require_seq| require_seq < after_seq);
        let breach = is_missing.then(|| vec![format!("{after} without {require} after it")]);
        evaluation.judge(id, check, breach);
    }
    evaluation
}

/// The evidence a judgment point carries: captures by name, `arg:<pointer>` from the anchor call's input.
#[must_use]
pub fn evidence(point: &Judgment, log: &AnchorLog, input: &Value) -> Vec<String> {
    point
        .evidence
        .iter()
        .map(|name| {
            let value = match name.strip_prefix(ARG_PREFIX) {
                Some(pointer) => resolve(input, pointer),
                None => log.latest_capture(name).cloned(),
            };
            format!(
                "{name} = {}",
                value.map_or_else(|| "absent".to_owned(), |value| value.to_string())
            )
        })
        .collect()
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use serde_json::json;

    use super::*;

    const DEMO: &str = include_str!("../tests/fixtures/enforce/heartbeat-demo.json");

    fn demo() -> EnforceDoc {
        EnforceDoc::parse(DEMO.as_bytes()).expect("demo enforce.json")
    }

    fn bash(command: &str) -> Value {
        json!({"command": command})
    }

    fn stage(value: &str) -> Value {
        json!({"scriptPath": "/h/skills/heartbeat-demo/workflows/demo.js", "args": {"value": value, "artifactsDir": "/a"}})
    }

    fn produce(log: &mut AnchorLog, doc: &EnforceDoc, seq: u64, value: &str) {
        let response =
            json!({"stdout": json!({"value": value}).to_string(), "stderr": ""}).to_string();
        log.record(
            doc,
            seq,
            true,
            "Bash",
            &bash("smine-rules demo-value --seed s1"),
            Some(&response),
        );
    }

    #[test]
    fn provenance_and_requires() {
        let doc = demo();
        let mut log = AnchorLog::default();
        let before = pre_tool(&doc, &log, true, "Workflow", &stage("abc"));
        assert_eq!(
            before.failure.map(|failure| failure.check),
            Some("producer_first".to_owned())
        );

        produce(&mut log, &doc, 1, "abc");
        assert_eq!(
            pre_tool(&doc, &log, true, "Workflow", &stage("abc")).failure,
            None
        );
        let laundered = pre_tool(&doc, &log, true, "Workflow", &stage("zzz"))
            .failure
            .expect("laundered value");
        assert_eq!(laundered.check, "value_provenance");

        let as_string = json!({"scriptPath": "/x/heartbeat-demo/workflows/demo.js", "args": "{\"value\":\"abc\"}"});
        assert_eq!(
            pre_tool(&doc, &log, true, "Workflow", &as_string).failure,
            None
        );
    }

    #[test]
    fn phase_and_stop() {
        let doc = demo();
        let mut log = AnchorLog::default();
        produce(&mut log, &doc, 1, "abc");
        log.record(&doc, 2, true, "Workflow", &stage("abc"), None);
        let edit = json!({"file_path": "/tmp/x"});
        assert_eq!(
            pre_tool(&doc, &log, true, "Write", &edit)
                .failure
                .map(|failure| failure.check),
            Some("no_edit_after_stage".to_owned())
        );
        assert_eq!(
            on_stop(&doc, &log).failure.map(|failure| failure.check),
            Some("close_before_stop".to_owned())
        );

        log.record(
            &doc,
            3,
            true,
            "Bash",
            &bash("smine-rules demo-close --dir /a"),
            None,
        );
        assert_eq!(pre_tool(&doc, &log, true, "Write", &edit).failure, None);
        assert_eq!(on_stop(&doc, &log).failure, None);
    }

    #[test]
    fn anchor_agent_scope() {
        let doc = demo();
        let mut log = AnchorLog::default();
        let matched = log.record(
            &doc,
            1,
            false,
            "Bash",
            &bash("smine-rules demo-value --seed s1"),
            None,
        );
        assert!(
            matched.is_empty(),
            "a subagent call never matches a root anchor"
        );
    }

    #[test]
    fn judgment_evidence() {
        let doc = demo();
        let mut log = AnchorLog::default();
        produce(&mut log, &doc, 1, "abc");
        let point = doc.judgment.get("value_check").expect("judgment");
        assert_eq!(
            evidence(point, &log, &stage("abc")),
            vec![
                "value = \"abc\"".to_owned(),
                "arg:/args/value = \"abc\"".to_owned()
            ]
        );
    }

    #[test]
    fn requires_since_window() {
        let doc = EnforceDoc::parse(
            json!({
                "schema": 1, "skill": "s", "version": "1.0.0",
                "anchors": {
                    "round": {"tool": "^Workflow$"},
                    "ledger": {"tool": "^Bash$", "input": [{"field": "command", "pattern": "^ledger$"}]}
                },
                "checks": {
                    "ledger_between": {"kind": "requires", "on": "round", "after": "ledger", "since": "round", "reason": "r", "recovery": "x"}
                }
            })
            .to_string()
            .as_bytes(),
        )
        .expect("inline doc");
        let round = json!({"scriptPath": "/w/round.js"});
        let mut log = AnchorLog::default();
        assert_eq!(
            pre_tool(&doc, &log, true, "Workflow", &round),
            Evaluation {
                passed: vec!["ledger_between".to_owned()],
                failure: None,
            },
            "no since yet: the first round owes nothing and counts as a pass"
        );

        log.record(&doc, 1, true, "Workflow", &round, None);
        assert_eq!(
            pre_tool(&doc, &log, true, "Workflow", &round)
                .failure
                .map(|failure| failure.check),
            Some("ledger_between".to_owned())
        );

        log.record(&doc, 2, true, "Bash", &bash("ledger"), None);
        assert_eq!(
            pre_tool(&doc, &log, true, "Workflow", &round).passed,
            ["ledger_between"]
        );
    }

    #[test]
    fn railroad_hunk_phase() {
        let doc = EnforceDoc::parse(
            include_str!("../tests/fixtures/enforce/railroad-review.json").as_bytes(),
        )
        .expect("railroad-review enforce.json");
        let mut log = AnchorLog::default();
        let checklist = json!({"scriptPath": "/h/skills/railroad-review/workflows/checklist.js"});
        log.record(&doc, 1, true, "Workflow", &checklist, None);
        assert_eq!(
            pre_tool(&doc, &log, true, "Bash", &bash("git diff a b -- x.go")).failure,
            None,
            "a checklist stage opens no phase"
        );

        log.record(
            &doc,
            2,
            true,
            "Bash",
            &bash("smine-rules lanes --round-dir r1"),
            None,
        );
        assert_eq!(
            pre_tool(&doc, &log, true, "Bash", &bash("git diff a b -- x.go"))
                .failure
                .map(|failure| failure.check),
            Some("no_hunks".to_owned())
        );
        assert_eq!(
            pre_tool(&doc, &log, true, "Bash", &bash("git diff --numstat a b")).failure,
            None
        );
    }

    #[test]
    fn every_guarded_check_is_judged() {
        let doc = demo();
        let mut log = AnchorLog::default();
        let skipped = pre_tool(&doc, &log, true, "Workflow", &stage("abc"));
        assert!(skipped.passed.is_empty(), "both stage checks broke");
        assert_eq!(
            skipped.failure.map(|failure| failure.check),
            Some("producer_first".to_owned()),
            "value_provenance broke too; the first breach in id order decides"
        );
        let edit = json!({"file_path": "/tmp/x"});
        assert_eq!(
            pre_tool(&doc, &log, true, "Write", &edit).passed,
            ["no_edit_after_stage"],
            "a phase that is not open passes"
        );
        assert_eq!(
            pre_tool(&doc, &log, true, "Read", &edit),
            Evaluation::default(),
            "no guard matched: nothing judged"
        );
        assert_eq!(
            pre_tool(&doc, &log, false, "Workflow", &stage("abc")),
            Evaluation::default(),
            "a subagent call never matches a root anchor"
        );
        assert_eq!(
            on_stop(&doc, &log),
            Evaluation::default(),
            "no stage yet: the stop check is not judged"
        );

        produce(&mut log, &doc, 1, "abc");
        let laundered = pre_tool(&doc, &log, true, "Workflow", &stage("zzz"));
        assert_eq!(
            laundered.passed,
            ["producer_first"],
            "a denied call still counts the checks it held"
        );
        assert_eq!(
            laundered.failure.map(|failure| failure.check),
            Some("value_provenance".to_owned())
        );

        log.record(&doc, 2, true, "Workflow", &stage("abc"), None);
        log.record(
            &doc,
            3,
            true,
            "Bash",
            &bash("smine-rules demo-close --dir /a"),
            None,
        );
        assert_eq!(
            on_stop(&doc, &log),
            Evaluation {
                passed: vec!["close_before_stop".to_owned()],
                failure: None,
            }
        );
    }

    #[test]
    fn rejects_other_schema() {
        let other = DEMO.replacen("\"schema\": 1", "\"schema\": 2", 1);
        assert!(EnforceDoc::parse(other.as_bytes()).is_err());
    }

    #[test]
    fn stdout_document_cases() {
        let value = json!({"value": "abc"});
        let cases = [
            ("pure-json", "{\"value\":\"abc\"}", Some(value.clone())),
            (
                "pure-json-padded",
                "  {\"value\":\"abc\"}\n\n",
                Some(value.clone()),
            ),
            (
                "pretty-printed",
                "{\n  \"value\": \"abc\"\n}\n",
                Some(value.clone()),
            ),
            (
                "leading-log-lines",
                "2026/10/02 01:41:07 shell: git dur=58ms err=false\n2026/10/02 01:41:07 shell: git dur=3ms err=false\n{\"value\":\"abc\"}\n",
                Some(value.clone()),
            ),
            (
                "leading-noise-pretty",
                "shell: git dur=58ms err=false\n{\n  \"value\": \"abc\"\n}\n",
                Some(value.clone()),
            ),
            (
                "noise-line-starts-with-brace",
                "{not json\n{\"value\":\"abc\"}",
                Some(value.clone()),
            ),
            (
                "crlf",
                "shell: git\r\n{\"value\":\"abc\"}\r\n",
                Some(value.clone()),
            ),
            ("array-tail", "shell: git\n[1, 2]", Some(json!([1, 2]))),
            ("trailing-noise", "{\"value\":\"abc\"}\ndone", None),
            (
                "trailing-json-scalar",
                "{\"value\":\"abc\"}\n42",
                Some(json!(42)),
            ),
            ("only-noise", "shell: git dur=58ms err=false\n", None),
            ("empty", "", None),
            ("whitespace", " \n\t\n", None),
        ];
        for (label, stdout, want) in cases {
            assert_eq!(stdout_document(stdout), want, "{label}");
        }
    }

    #[test]
    fn capture_after_log_lines() {
        let doc = demo();
        let mut log = AnchorLog::default();
        let noisy = json!({
            "stdout": "2026/10/02 01:41:07 shell: git dur=58ms err=false\n{\n  \"value\": \"abc\"\n}\n",
            "stderr": ""
        })
        .to_string();
        log.record(
            &doc,
            1,
            true,
            "Bash",
            &bash("smine-rules demo-value --seed s1"),
            Some(&noisy),
        );
        assert_eq!(
            pre_tool(&doc, &log, true, "Workflow", &stage("abc")).failure,
            None,
            "the JSON after the log line is the capture"
        );

        let mut voided = AnchorLog::default();
        let tail = json!({"stdout": "{\"value\":\"abc\"}\ndone", "stderr": ""}).to_string();
        voided.record(
            &doc,
            1,
            true,
            "Bash",
            &bash("smine-rules demo-value --seed s1"),
            Some(&tail),
        );
        let entry = voided.last_entry("produced").expect("the call is logged");
        assert!(
            entry.captures.is_empty(),
            "trailing noise: the entry holds no value"
        );
        let failure = pre_tool(&doc, &voided, true, "Workflow", &stage("abc"))
            .failure
            .expect("the producing call yielded no value");
        assert_eq!(failure.check, "value_provenance");
        assert_eq!(
            failure.evidence,
            [
                "/args/value = \"abc\"".to_owned(),
                "produced.value = absent".to_owned()
            ]
        );
    }

    #[test]
    fn provenance_reads_the_producing_call() {
        let doc = demo();
        let noise = json!({"stdout": "{\"value\":\"abc\"}\ndone", "stderr": ""}).to_string();
        let cases = [
            (
                "later-call-without-value-denies",
                Some(noise.as_str()),
                Some(vec![
                    "/args/value = \"abc\"".to_owned(),
                    "produced.value = absent".to_owned(),
                ]),
            ),
            ("same-value-twice-passes", None, None),
        ];
        for (label, second, want) in cases {
            let mut log = AnchorLog::default();
            produce(&mut log, &doc, 1, "abc");
            match second {
                Some(response) => {
                    log.record(
                        &doc,
                        2,
                        true,
                        "Bash",
                        &bash("smine-rules demo-value --seed s1"),
                        Some(response),
                    );
                }
                None => produce(&mut log, &doc, 2, "abc"),
            }
            let got = pre_tool(&doc, &log, true, "Workflow", &stage("abc"))
                .failure
                .map(|failure| failure.evidence);
            assert_eq!(got, want, "{label}");
        }
    }

    #[test]
    fn judgment_evidence_reads_latest_capture() {
        let doc = demo();
        let mut log = AnchorLog::default();
        produce(&mut log, &doc, 1, "abc");
        produce(&mut log, &doc, 2, "def");
        let point = doc.judgment.get("value_check").expect("judgment");
        assert_eq!(
            evidence(point, &log, &stage("def")),
            vec![
                "value = \"def\"".to_owned(),
                "arg:/args/value = \"def\"".to_owned()
            ]
        );
    }
}
