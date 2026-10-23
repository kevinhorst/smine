//! The heart attack: heartbeat itself stops every subagent that runs on a forbidden model — no orchestrator, no agent
//! in the loop (`forbidden_subagent_models` in the heartbeat config). Subagents on allowed models are never touched.
//!
//! Sightings:
//! * a `PreToolUse` of the `Agent` tool whose `model` input is forbidden — the call itself is denied, nothing spawns;
//! * a registered subagent's transcript showing a forbidden model — while a subagent's model is unknown, each of its
//!   tool hooks and its `SubagentStop` ask the transcript edge to read it ([`Action::ReadAgentModel`]);
//! * a registered subagent whose spawn named no model (inherited), or whose transcript model does not contain the
//!   model its spawn named (substituted by the harness) — checked after the forbidden check.
//!
//! Enforcement is the `/hook` answer, decided by [`verdict`] right after the hook was applied: every `PreToolUse`,
//! `PostToolUse` and `PostToolUseFailure` of a registered subagent whose model is forbidden is answered with
//! `continue: false`, which ends that subagent's agent loop (hooks doc, Decision control). The stop lands at the
//! subagent's next tool hook after its model was read: one inside a long tool call stops when the call returns. The
//! root session is never stopped; an `agent_id` no `SubagentStart` registered (an `--agent` session's root) is never
//! stopped. The subagent's recorded model is the one source of truth — no separate arrest state.
//!
//! Pure, like `apply`: no clock, no I/O.

use serde_json::Value;

use crate::action::{Action, ModelRead};
use crate::event::{HookEvent, HookKind, ToolCall};
use crate::fleet::{Fleet, Requested};
use crate::ids::{AgentId, SessionId};
use crate::protocol::AGENT_TOOL;

/// What the `/hook` answer says.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Verdict {
    /// Proceed: `{}`.
    Allow,
    /// An `Agent` call asked for a forbidden model: the call is denied.
    Deny {
        /// The `permissionDecisionReason`.
        reason: String,
    },
    /// A tool hook of a subagent on a forbidden model: its agent loop ends.
    Stop {
        /// The `stopReason` and the `systemMessage` (and, before a tool call, the `permissionDecisionReason`).
        reason: String,
        /// A `PreToolUse`: the pending call is denied as well.
        pre_tool: bool,
    },
    /// `additionalContext` for the hook that asked: a joined member's prompt ([`crate::member`]) or a runtime
    /// hand-over on a tool call ([`crate::runtime`]).
    Context {
        /// The hook the context answers.
        event: ContextEvent,
        /// The context text.
        text: String,
    },
    /// A root `Stop` an enforce check refuses: `{"decision": "block", "reason": …}`, the agent continues.
    Block {
        /// The `reason` the agent reads.
        reason: String,
    },
}

/// The hook a `Context` verdict answers — its `hookEventName`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ContextEvent {
    /// A root prompt: the member guard's role reminder.
    UserPromptSubmit,
    /// A successful tool call: a runtime template hand-over.
    PostToolUse,
}

impl ContextEvent {
    /// The `hookEventName` string of the hook answer.
    #[must_use]
    pub fn hook_event_name(self) -> &'static str {
        match self {
            ContextEvent::UserPromptSubmit => "UserPromptSubmit",
            ContextEvent::PostToolUse => "PostToolUse",
        }
    }
}

/// What [`verdict`] needs of a hook, taken before `apply` consumes it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Probe {
    /// `session_id`.
    session: SessionId,
    /// `agent_id`.
    agent: Option<AgentId>,
    /// The hook, reduced to what can carry a verdict.
    hook: ProbedHook,
}

/// The hooks a verdict can apply to.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProbedHook {
    /// `PreToolUse`: the tool, and the `model` input when the tool is `Agent`.
    PreTool {
        /// `tool_name`.
        tool_name: String,
        /// `tool_input.model` of an `Agent` call.
        requested_model: Option<String>,
    },
    /// `PostToolUse` (`is_failure` false) or `PostToolUseFailure` (`is_failure` true).
    PostTool {
        /// A `PostToolUseFailure`.
        is_failure: bool,
    },
    /// `UserPromptSubmit`.
    Prompt,
    /// `Stop`: the agent ends its turn.
    Stop,
    /// Any other hook: always allowed.
    Other,
}

impl Probe {
    /// The probe of `hook`.
    #[must_use]
    pub fn of(hook: &HookEvent) -> Probe {
        let probed = match &hook.kind {
            HookKind::PreToolUse(call) => ProbedHook::PreTool {
                tool_name: call.tool_name.clone(),
                requested_model: requested_model(call).map(str::to_owned),
            },
            HookKind::PostToolUse(_) => ProbedHook::PostTool { is_failure: false },
            HookKind::PostToolUseFailure(_) => ProbedHook::PostTool { is_failure: true },
            HookKind::UserPromptSubmit { .. } => ProbedHook::Prompt,
            HookKind::Stop => ProbedHook::Stop,
            HookKind::SessionStart { .. }
            | HookKind::SessionEnd { .. }
            | HookKind::PermissionRequest(_)
            | HookKind::Notification { .. }
            | HookKind::StopFailure { .. }
            | HookKind::SubagentStart
            | HookKind::SubagentStop
            | HookKind::Other { .. } => ProbedHook::Other,
        };
        Probe {
            session: hook.session_id.clone(),
            agent: hook.agent.as_ref().map(|agent| agent.id.clone()),
            hook: probed,
        }
    }

    /// `session_id`.
    #[must_use]
    pub fn session(&self) -> &SessionId {
        &self.session
    }

    /// `agent_id`.
    #[must_use]
    pub fn agent(&self) -> Option<&AgentId> {
        self.agent.as_ref()
    }

    /// The probed hook.
    #[must_use]
    pub fn hook(&self) -> &ProbedHook {
        &self.hook
    }
}

/// A registered subagent's tool hook or stop asks for its model while that is unknown.
pub(crate) fn on_hook(fleet: &Fleet, hook: &HookEvent) -> Vec<Action> {
    match &hook.kind {
        HookKind::PreToolUse(_)
        | HookKind::PostToolUse(_)
        | HookKind::PostToolUseFailure(_)
        | HookKind::SubagentStop => model_read(fleet, hook),
        HookKind::SessionStart { .. }
        | HookKind::SessionEnd { .. }
        | HookKind::UserPromptSubmit { .. }
        | HookKind::PermissionRequest(_)
        | HookKind::Notification { .. }
        | HookKind::Stop
        | HookKind::StopFailure { .. }
        | HookKind::SubagentStart
        | HookKind::Other { .. } => Vec::new(),
    }
}

/// Records a subagent's model and request, read from its transcript and `meta.json`. A read that found no model, a
/// read for an unknown session or subagent, or a subagent whose model is already known changes nothing.
pub(crate) fn on_model_read(
    fleet: &mut Fleet,
    session: &SessionId,
    agent: &AgentId,
    model: Option<String>,
    requested: Requested,
) {
    let Some(model) = model else {
        return;
    };
    let subagent = fleet
        .sessions
        .get_mut(session)
        .and_then(|host| host.subagents.get_mut(agent));
    if let Some(subagent) = subagent.filter(|subagent| subagent.model.is_none()) {
        subagent.model = Some(model);
        subagent.requested = requested;
    }
}

/// The `/hook` answer for the probed hook, read from the fleet after the hook was applied.
#[must_use]
pub fn verdict(fleet: &Fleet, probe: &Probe) -> Verdict {
    let texts = &fleet.runtime.texts;
    let pre_tool = match &probe.hook {
        ProbedHook::Other | ProbedHook::Prompt | ProbedHook::Stop => return Verdict::Allow,
        ProbedHook::PostTool { .. } => false,
        ProbedHook::PreTool {
            requested_model, ..
        } => {
            if let Some(model) = requested_model
                .as_deref()
                .filter(|model| fleet.forbidden_models.matches(model))
            {
                return Verdict::Deny {
                    reason: texts.attack_deny_reason().replace("{model}", model),
                };
            }
            true
        }
    };
    let Some(subagent) = probe
        .agent
        .as_ref()
        .and_then(|agent| fleet.sessions.get(&probe.session)?.subagents.get(agent))
    else {
        return Verdict::Allow;
    };
    let Some(model) = subagent.model.as_deref() else {
        return Verdict::Allow;
    };
    let reason = if fleet.forbidden_models.matches(model) {
        texts.attack_stop_reason().replace("{model}", model)
    } else {
        match &subagent.requested {
            Requested::Inherited => texts.attack_inherited_reason().replace("{model}", model),
            Requested::Named(requested) if !resolves(requested, model) => texts
                .attack_substituted_reason()
                .replace("{requested}", requested)
                .replace("{model}", model),
            Requested::Named(_) | Requested::Unknown => return Verdict::Allow,
        }
    };
    Verdict::Stop { reason, pre_tool }
}

/// Whether a subagent that named `requested` runs on `model`: `model` contains the request, compared lowercase with
/// the request's `[1m]` context suffix stripped (`sonnet` in `claude-sonnet-5-5`, `claude-opus-5-5[1m]` in
/// `claude-opus-5-5`).
#[must_use]
fn resolves(requested: &str, model: &str) -> bool {
    let requested = requested.trim().to_ascii_lowercase();
    let requested = requested.trim_end_matches("[1m]");
    model.to_ascii_lowercase().contains(requested)
}

/// `tool_input.model` of an `Agent` call.
fn requested_model(call: &ToolCall) -> Option<&str> {
    if call.tool_name != AGENT_TOOL {
        return None;
    }
    call.input.get("model").and_then(Value::as_str)
}

/// A read of the subagent's transcript head while its model is unknown; nothing for root events, unregistered agents
/// or a hook without `transcript_path`.
fn model_read(fleet: &Fleet, hook: &HookEvent) -> Vec<Action> {
    let (Some(agent), Some(transcript)) = (&hook.agent, &hook.transcript_path) else {
        return Vec::new();
    };
    let unread = fleet
        .sessions
        .get(&hook.session_id)
        .and_then(|session| session.subagents.get(&agent.id))
        .is_some_and(|subagent| subagent.model.is_none());
    if !unread {
        return Vec::new();
    }
    vec![Action::ReadAgentModel(ModelRead {
        session: hook.session_id.clone(),
        agent: agent.id.clone(),
        transcript: transcript.clone(),
    })]
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use jiff::Timestamp;
    use serde_json::json;

    use super::*;
    use crate::apply::apply;
    use crate::config::ForbiddenModels;
    use crate::event::{Agent, Event};

    const S: &str = "s";
    const TRANSCRIPT: &str = "/home/fx/.claude/projects/-repo/s.jsonl";

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    fn hook(agent: Option<&str>, second: i64, kind: HookKind) -> HookEvent {
        HookEvent {
            session_id: SessionId(S.to_owned()),
            agent: agent.map(|id| Agent {
                id: AgentId(id.to_owned()),
                agent_type: "Plan".to_owned(),
            }),
            cwd: None,
            transcript_path: Some(TRANSCRIPT.into()),
            at: at(second),
            kind,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        }
    }

    fn tool(name: &str, input: Value) -> ToolCall {
        ToolCall {
            tool_name: name.to_owned(),
            input,
        }
    }

    fn fleet(patterns: &[&str]) -> Fleet {
        let mut fleet = Fleet::new(at(0));
        let patterns: Vec<String> = patterns
            .iter()
            .map(|pattern| (*pattern).to_owned())
            .collect();
        fleet.forbidden_models = ForbiddenModels::new(&patterns).expect("patterns compile");
        fleet
    }

    /// Applies `hook` the way the engine does; its verdict and actions.
    fn answer(fleet: &mut Fleet, hook: HookEvent) -> (Verdict, Vec<Action>) {
        let probe = Probe::of(&hook);
        let actions = apply(fleet, Event::Hook(hook));
        (verdict(fleet, &probe), actions)
    }

    fn model_read(
        fleet: &mut Fleet,
        agent: &str,
        model: Option<&str>,
        requested: Requested,
        second: i64,
    ) {
        let _ = apply(
            fleet,
            Event::AgentModelRead {
                session: SessionId(S.to_owned()),
                agent: AgentId(agent.to_owned()),
                model: model.map(str::to_owned),
                requested,
                at: at(second),
            },
        );
    }

    fn named(model: &str) -> Requested {
        Requested::Named(model.to_owned())
    }

    /// Starts the session at second 1 and registers `agents` at their seconds.
    fn started(fleet: &mut Fleet, agents: &[(&str, i64)]) {
        let start = HookKind::SessionStart {
            source: "startup".to_owned(),
        };
        let _ = apply(fleet, Event::Hook(hook(None, 1, start)));
        for (agent, second) in agents {
            let _ = apply(
                fleet,
                Event::Hook(hook(Some(*agent), *second, HookKind::SubagentStart)),
            );
        }
    }

    fn is_stop(verdict: &Verdict, before_tool: bool) -> bool {
        matches!(verdict, Verdict::Stop { pre_tool, .. } if *pre_tool == before_tool)
    }

    #[test]
    fn only_the_forbidden_subagent_is_stopped() {
        let mut fleet = fleet(&["*fable*"]);
        started(&mut fleet, &[("a1", 2), ("a2", 3)]);
        let read = tool("Read", json!({ "file_path": "/r" }));
        let (first, actions) = answer(&mut fleet, hook(Some("a2"), 4, HookKind::PreToolUse(read)));
        assert_eq!(first, Verdict::Allow, "the model is not known yet");
        let want = Action::ReadAgentModel(ModelRead {
            session: SessionId(S.to_owned()),
            agent: AgentId("a2".to_owned()),
            transcript: TRANSCRIPT.into(),
        });
        assert!(actions.contains(&want), "{actions:?}");
        model_read(&mut fleet, "a1", Some("claude-opus-5-5"), named("opus"), 5);
        model_read(&mut fleet, "a2", Some("claude-fable-5-1"), named("opus"), 5);
        let done = tool("Read", json!({}));
        let (offender, _) = answer(&mut fleet, hook(Some("a2"), 6, HookKind::PostToolUse(done)));
        assert!(is_stop(&offender, false), "{offender:?}");
        let ls = tool("Bash", json!({ "command": "ls" }));
        let (sibling, _) = answer(
            &mut fleet,
            hook(Some("a1"), 6, HookKind::PreToolUse(ls.clone())),
        );
        assert_eq!(
            sibling,
            Verdict::Allow,
            "a sibling on an allowed model runs"
        );
        let (root, _) = answer(&mut fleet, hook(None, 7, HookKind::PreToolUse(ls)));
        assert_eq!(root, Verdict::Allow, "the root session is never stopped");
    }

    #[test]
    fn forbidden_agent_call_is_denied_and_leaves_running_subagents_alone() {
        let mut fleet = fleet(&["*fable*"]);
        started(&mut fleet, &[("a1", 2)]);
        model_read(
            &mut fleet,
            "a1",
            Some("claude-sonnet-5"),
            named("sonnet"),
            2,
        );
        let spawn = tool(
            "Agent",
            json!({ "subagent_type": "Plan", "prompt": "p", "model": "fable" }),
        );
        let (denied, _) = answer(&mut fleet, hook(None, 3, HookKind::PreToolUse(spawn)));
        assert!(
            matches!(&denied, Verdict::Deny { reason } if reason.contains("fable")),
            "{denied:?}"
        );
        let failed = tool("Bash", json!({ "command": "false" }));
        let (sibling, _) = answer(
            &mut fleet,
            hook(Some("a1"), 4, HookKind::PostToolUseFailure(failed)),
        );
        assert_eq!(sibling, Verdict::Allow);
    }

    #[test]
    fn unregistered_agents_run() {
        let mut fleet = fleet(&["*fable*"]);
        started(&mut fleet, &[]);
        let read = tool("Read", json!({}));
        let (got, actions) = answer(
            &mut fleet,
            hook(Some("agent-root"), 5, HookKind::PreToolUse(read)),
        );
        assert_eq!(got, Verdict::Allow);
        assert!(
            actions.is_empty(),
            "no read for an unregistered agent: {actions:?}"
        );
    }

    #[test]
    fn allowed_or_unknown_models_never_stop() {
        let cases: [(&str, &[&str], Option<&str>, &str); 3] = [
            (
                "allowed-model",
                &["*fable*"],
                Some("claude-opus-5-5"),
                "opus",
            ),
            ("no-policy", &[], Some("claude-fable-5-1"), "fable"),
            ("no-model-yet", &["*fable*"], None, "opus"),
        ];
        for (label, patterns, model, requested) in cases {
            let mut fleet = fleet(patterns);
            started(&mut fleet, &[("a1", 2)]);
            model_read(&mut fleet, "a1", model, named(requested), 3);
            let done = tool("Read", json!({}));
            let (got, actions) =
                answer(&mut fleet, hook(Some("a1"), 4, HookKind::PostToolUse(done)));
            assert_eq!(got, Verdict::Allow, "{label}");
            let reads = actions
                .iter()
                .any(|action| matches!(action, Action::ReadAgentModel(_)));
            assert_eq!(
                reads,
                model.is_none(),
                "{label}: a model is read until known"
            );
        }
    }

    #[test]
    fn resolves_matches_the_probe() {
        let cases: [(&str, &str, bool); 7] = [
            ("sonnet", "claude-sonnet-5-5", true),
            ("claude-opus-5-5[1m]", "claude-opus-5-5", true),
            ("Sonnet", "claude-sonnet-5-5", true),
            ("haiku", "claude-fable-5-1", false),
            ("sonnet5", "claude-fable-5-1", false),
            ("sonnet-5", "claude-fable-5-1", false),
            ("claude-haiku-4-5-20251001", "claude-fable-5-1", false),
        ];
        for (requested, model, want) in cases {
            assert_eq!(resolves(requested, model), want, "{requested} / {model}");
        }
    }

    #[test]
    fn inherited_and_substituted_subagents_are_stopped() {
        let mut fleet = fleet(&[]);
        started(&mut fleet, &[("a1", 2), ("a2", 2), ("a3", 2), ("a4", 2)]);
        let opus = Some("claude-opus-5-5");
        model_read(&mut fleet, "a1", opus, named("haiku"), 3);
        model_read(&mut fleet, "a2", opus, Requested::Inherited, 3);
        model_read(&mut fleet, "a3", opus, named("opus"), 3);
        model_read(&mut fleet, "a4", opus, Requested::Unknown, 3);
        let ls = tool("Bash", json!({ "command": "ls" }));
        let mut ask = |agent: Option<&str>| {
            answer(&mut fleet, hook(agent, 4, HookKind::PreToolUse(ls.clone()))).0
        };
        let substituted = ask(Some("a1"));
        assert!(
            matches!(&substituted, Verdict::Stop { reason, pre_tool: true }
                if reason.contains("haiku") && reason.contains("claude-opus-5-5")),
            "{substituted:?}"
        );
        let inherited = ask(Some("a2"));
        assert!(
            matches!(&inherited, Verdict::Stop { reason, pre_tool: true }
                if reason.contains("without a model")),
            "{inherited:?}"
        );
        assert_eq!(ask(Some("a3")), Verdict::Allow, "named and resolving");
        assert_eq!(ask(Some("a4")), Verdict::Allow, "no meta.json: no judgment");
        assert_eq!(
            ask(None),
            Verdict::Allow,
            "the root session is never stopped"
        );
    }

    #[test]
    fn forbidden_wins() {
        let mut fleet = fleet(&["*fable*"]);
        started(&mut fleet, &[("a1", 2)]);
        model_read(
            &mut fleet,
            "a1",
            Some("claude-fable-5-1"),
            named("haiku"),
            3,
        );
        let done = tool("Read", json!({}));
        let (got, _) = answer(&mut fleet, hook(Some("a1"), 4, HookKind::PostToolUse(done)));
        assert!(
            matches!(&got, Verdict::Stop { reason, pre_tool: false }
                if reason.starts_with("heartbeat heart attack:")),
            "{got:?}"
        );
    }
}
