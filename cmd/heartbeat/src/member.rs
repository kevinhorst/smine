//! Fleet-member guard: a joined member's one channel to its orchestrator is `/done`. Its `SendMessage` calls (its
//! subagents' included) are denied, and each prompt its root receives — typed, or an orchestrator message — carries
//! its role as `additionalContext`. Evaluated after the heart attack in the `/hook` answer. Pure, like `attack`.

use crate::attack::{ContextEvent, Probe, ProbedHook, Verdict};
use crate::fleet::Fleet;
use crate::protocol::SEND_MESSAGE_TOOL;

/// The `/hook` answer for a joined member's hook; `Allow` for every other session and hook. The deny reason and the
/// role text are the config's `member_deny_reason` and `member_role_text`.
#[must_use]
pub fn verdict(fleet: &Fleet, probe: &Probe) -> Verdict {
    let texts = &fleet.runtime.texts;
    let Some(join) = fleet.joins.get(probe.session()) else {
        return Verdict::Allow;
    };
    let key = (probe.session().clone(), None);
    let Some(token) = fleet
        .member_id(&join.fleet, &key)
        .map(|letter| letter.token(&join.fleet))
    else {
        return Verdict::Allow;
    };
    match probe.hook() {
        ProbedHook::PreTool { tool_name, .. } if tool_name == SEND_MESSAGE_TOOL => Verdict::Deny {
            reason: texts.member_deny_reason().replace("{token}", &token),
        },
        ProbedHook::Prompt if probe.agent().is_none() => Verdict::Context {
            event: ContextEvent::UserPromptSubmit,
            text: texts
                .member_role_text()
                .replace("{token}", &token)
                .replace("{fleet}", &join.fleet.0),
        },
        ProbedHook::PreTool { .. }
        | ProbedHook::PostTool { .. }
        | ProbedHook::Prompt
        | ProbedHook::Stop
        | ProbedHook::Other => Verdict::Allow,
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use jiff::Timestamp;
    use serde_json::json;

    use super::*;
    use crate::event::{Agent, HookEvent, HookKind, ToolCall};
    use crate::fleet::{FleetInstance, Join, Session, SessionState};
    use crate::ids::{AgentId, FleetId, InstanceId, MemberId, MessagingToken, SessionId};

    const MEMBER: &str = "m";
    const OTHER: &str = "o";

    fn fleet() -> Fleet {
        let at = Timestamp::UNIX_EPOCH;
        let mut fleet = Fleet::new(at);
        let name = FleetId("T1".to_owned());
        let mut instance = FleetInstance::new(InstanceId(uuid::Uuid::nil()), name.clone(), at);
        instance
            .member_ids
            .by_member
            .insert((SessionId(MEMBER.to_owned()), None), MemberId::nth(2));
        fleet.instances.insert(name.clone(), instance);
        let join = Join {
            fleet: name,
            token: MessagingToken("t".to_owned()),
            since: at,
        };
        fleet.joins.insert(SessionId(MEMBER.to_owned()), join);
        for id in [MEMBER, OTHER] {
            let session = Session::new(SessionId(id.to_owned()), SessionState::Idle, at, false);
            fleet.sessions.insert(SessionId(id.to_owned()), session);
        }
        fleet
    }

    fn probe(session: &str, agent: Option<&str>, kind: HookKind) -> Probe {
        let hook = HookEvent {
            session_id: SessionId(session.to_owned()),
            agent: agent.map(|id| Agent {
                id: AgentId(id.to_owned()),
                agent_type: String::new(),
            }),
            cwd: None,
            transcript_path: None,
            at: Timestamp::UNIX_EPOCH,
            kind,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        };
        Probe::of(&hook)
    }

    fn pre_tool(name: &str) -> HookKind {
        HookKind::PreToolUse(ToolCall {
            tool_name: name.to_owned(),
            input: json!({}),
        })
    }

    fn prompt() -> HookKind {
        HookKind::UserPromptSubmit {
            prompt: "go".to_owned(),
        }
    }

    #[test]
    fn member_verdict() {
        let fleet = fleet();
        let is_deny = |verdict: &Verdict| matches!(verdict, Verdict::Deny { reason } if reason.contains("T1-MC is a fleet member"));
        let is_context = |verdict: &Verdict| matches!(verdict, Verdict::Context { event: ContextEvent::UserPromptSubmit, text } if text.contains("T1-MC") && text.contains("fleet T1"));
        let cases = [
            (
                "member-sendmessage-denied",
                probe(MEMBER, None, pre_tool("SendMessage")),
                'd',
            ),
            (
                "member-subagent-sendmessage-denied",
                probe(MEMBER, Some("a1"), pre_tool("SendMessage")),
                'd',
            ),
            (
                "non-member-sendmessage-allowed",
                probe(OTHER, None, pre_tool("SendMessage")),
                'a',
            ),
            ("member-prompt-context", probe(MEMBER, None, prompt()), 'c'),
            (
                "member-subagent-prompt-allowed",
                probe(MEMBER, Some("a1"), prompt()),
                'a',
            ),
            (
                "non-member-prompt-allowed",
                probe(OTHER, None, prompt()),
                'a',
            ),
            (
                "member-other-tool-allowed",
                probe(MEMBER, None, pre_tool("Bash")),
                'a',
            ),
        ];
        for (label, probe, want) in cases {
            let got = verdict(&fleet, &probe);
            let is_match = match want {
                'd' => is_deny(&got),
                'c' => is_context(&got),
                _ => got == Verdict::Allow,
            };
            assert!(is_match, "{label}: {got:?}");
        }
    }
}
