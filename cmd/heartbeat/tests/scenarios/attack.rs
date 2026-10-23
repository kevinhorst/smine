//! Heart attack scenarios against the built daemon: a subagent seen on a forbidden model is stopped through the hook
//! answers; subagents on allowed models and the root session run on.

use serde_json::{Value, json};

use crate::common::{Daemon, Home, row};

/// Starts the daemon with `forbidden_subagent_models: ["*fable*"]`.
fn start(home: &Home) -> Daemon {
    let args = home.config_args(&json!({ "forbidden_subagent_models": ["*fable*"] }));
    let refs: Vec<&str> = args.iter().map(String::as_str).collect();
    Daemon::start_with(home.path(), &refs)
}

/// Starts session `s1` and registers `agents` as its subagents.
fn session_with(daemon: &Daemon, home: &Home, agents: &[&str]) {
    assert_eq!(daemon.post(&home.root_hook("SessionStart", "s1")), 200);
    for agent in agents {
        assert_eq!(
            daemon.post(&home.agent_hook("SubagentStart", "s1", agent)),
            200
        );
    }
}

/// Posts `agent`'s first tool hook, which asks for its model, and waits until the daemon recorded `model`.
fn read_model(daemon: &Daemon, home: &Home, agent: &str, model: &str) {
    home.subagent_transcript("s1", agent, model);
    let (_, first) = daemon.post_json(&home.agent_hook("PreToolUse", "s1", agent));
    assert_eq!(first, json!({}), "the model is read after this answer");
    daemon.wait_for("the subagent's model", |doc| {
        row(doc, "s1").is_some_and(|session| {
            session["subagents"].as_array().is_some_and(|subagents| {
                subagents
                    .iter()
                    .any(|sub| sub["agent_id"] == agent && sub["model"] == model)
            })
        })
    });
}

#[test]
fn only_the_forbidden_subagent_is_stopped() {
    let home = Home::new();
    let daemon = start(&home);
    session_with(&daemon, &home, &["a1", "a2"]);
    read_model(&daemon, &home, "a1", "claude-opus-5-5");
    read_model(&daemon, &home, "a2", "claude-fable-5-1");
    let (status, offender) = daemon.post_json(&home.agent_hook("PostToolUse", "s1", "a2"));
    assert_eq!(status, 200);
    assert_eq!(offender["continue"], false);
    let (_, again) = daemon.post_json(&home.agent_hook("PreToolUse", "s1", "a2"));
    assert_eq!(again["continue"], false);
    assert_eq!(again["hookSpecificOutput"]["permissionDecision"], "deny");
    let (_, sibling) = daemon.post_json(&home.agent_hook("PreToolUse", "s1", "a1"));
    assert_eq!(sibling, json!({}), "a sibling on an allowed model runs");
    let (_, root) = daemon.post_json(&home.root_hook("PreToolUse", "s1"));
    assert_eq!(root, json!({}), "the root session is never stopped");
}

#[test]
fn forbidden_agent_call_is_denied() {
    let home = Home::new();
    let daemon = start(&home);
    session_with(&daemon, &home, &["a1"]);
    read_model(&daemon, &home, "a1", "claude-sonnet-5");
    let mut spawn: Value =
        serde_json::from_str(&home.root_hook("PreToolUse", "s1")).expect("hook body");
    spawn["tool_name"] = json!("Agent");
    spawn["tool_input"] =
        json!({ "subagent_type": "Plan", "description": "d", "prompt": "p", "model": "fable" });
    let (_, denied) = daemon.post_json(&spawn.to_string());
    assert_eq!(denied["hookSpecificOutput"]["permissionDecision"], "deny");
    let (_, sibling) = daemon.post_json(&home.agent_hook("PostToolUse", "s1", "a1"));
    assert_eq!(
        sibling,
        json!({}),
        "a running subagent on an allowed model runs"
    );
}

#[test]
fn without_a_policy_nothing_stops() {
    let home = Home::new();
    let daemon = Daemon::start(home.path());
    session_with(&daemon, &home, &["a1"]);
    read_model(&daemon, &home, "a1", "claude-fable-5-1");
    let (_, answer) = daemon.post_json(&home.agent_hook("PostToolUse", "s1", "a1"));
    assert_eq!(answer, json!({}));
}
