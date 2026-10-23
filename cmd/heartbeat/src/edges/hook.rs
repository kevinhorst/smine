//! The HTTP edge: `POST /hook` (every hook event; body = the hook's JSON input; answered 2xx once the event is
//! applied, with `{}` or the heart attack's deny or stop — [`crate::attack`]), `GET /healthz`
//! (identity), `GET /status` (the status document, `session` / `repo` / `fleet` query), `POST /subscribe` (a session
//! asks for digests; answered with its subscription row once the engine applied it), `POST /unsubscribe` (the session
//! ends its subscription with its token; 200 with the ended row, 409 while none is ended for it), `POST /join` (the
//! session joins a fleet; 200 with its member row, 409 with the refusal), `POST /done` (a joined session reports done
//! with its token; a session without a join is accepted for its branch's landing target), `POST /land`,
//! `POST /land/end` and `POST /land/release` (request, end or — the operator, no token — release a landing turn; 200
//! with the land row, 409 with the refusal), `POST /leave` (a joined session leaves its fleet with its token; 200 with
//! the member row it had), `POST /remove` (the operator removes a lettered member, no token — loopback trust like
//! `/status`), `POST /journal` (the fleet's active subscriber records an act with its token) and `POST /worker` (the
//! fleet's active subscriber registers a worker branch or reports it delivered, with its token). A join or subscribe
//! carries a
//! fresh UUID v7 minted here: the id of the fleet run it starts, if it starts one.
//! A hook that opens a skill run gets the skill's deployed `enforce.json` attached (the only file this edge reads per
//! hook); a root `Stop` an enforce check refuses is answered `{"decision": "block"}`.
//! Runtime enforcement adds `GET /runtime` (the runtime document), `POST /runtime/subscribe` (the one runtime
//! subscriber, socket-checked like `/subscribe`; 200 with its row, 409 with the refusal), `POST /runtime/unsubscribe`
//! (token-checked) and `POST /runtime/act` (the subscriber answers a finding with a move; 200 with the finding row once
//! the move's claim is on disk, 409 with the refusal or why the move is void).
//! Handlers parse and forward and hold no fleet state. A hook is answered only after its event was applied.
//! Answers: 200 with the verdict body, 400 unparseable body, 409 another profile's session, 503 engine gone — every
//! non-2xx is non-blocking for Claude Code and shows as a hook error notice (hooks doc L844).

use std::path::{Path, PathBuf};
use std::sync::Arc;

use axum::body::Bytes;
use axum::extract::{DefaultBodyLimit, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use jiff::Timestamp;
use serde::Deserialize;
use serde_json::{Value, json};
use tokio::sync::{mpsc::Sender, oneshot};

use crate::apply::slash_skill;
use crate::attack::Verdict;
use crate::config::{Caps, Startup};
use crate::edges::recorder::Recorded;
use crate::edges::sensor;
use crate::enforce::EnforceDoc;
use crate::engine::Input;
use crate::event::{Event, HookEvent, HookKind, JoinSource};
use crate::fleet::{
    Join, JournalEntry, JournalKind, SensorName, SensorState, SubscribeRequest, SubscriptionSource,
};
use crate::identity::Identity;
use crate::ids::{FleetId, InstanceId, MemberId, MessagingToken, SessionId, WorkerBranch};
use crate::land::{LandAnswer, LandCaller, LandEnd, LandSource, LandTarget};
use crate::log;
use crate::protocol::{ENFORCE_FILE, FLEET_ID_CHARSET, HOOK_PATH, MAX_MEMBER_LETTERS, SKILL_TOOL};
use crate::runtime::{
    self, ActAnswer, ActRequest, FindingId, RuntimeSubscribeRequest, SubscriptionAnswer,
};
use crate::status::Filter;
use crate::switches;
use crate::wire::{
    DoneAnswer, DoneBody, Health, JoinBody, JournalBody, JournalRow, LandAnswerRow, LandBody,
    LandEndBody, LandReleaseBody, LeaveBody, MemberRow, RecordedRow, RemoveBody, RuntimeActBody,
    RuntimeSubscribeBody, RuntimeUnsubscribeBody, StatusDocument, SubscribeBody, SubscriptionRow,
    SubscriptionStateWire, UnsubscribeBody, ViaWire, WorkerBody, WorkerStateWire,
};

/// What every handler shares: handles and start-time config, no fleet state.
#[derive(Debug, Clone)]
pub struct HookState {
    /// The engine inbox.
    pub inbox: Sender<Input>,
    /// This daemon's identity.
    pub identity: Arc<Identity>,
    /// The recorder, when `--record` is set.
    pub recorder: Option<std::sync::mpsc::Sender<Recorded>>,
    /// The config's start-time keys: body caps, the worker branch prefix, the claim poll.
    pub startup: Startup,
    /// The config's caps as read at start: the field and id lengths the shape checks apply.
    pub caps: Caps,
}

impl HookState {
    /// 1-`max_field` printable ASCII characters: session ids, tokens, findings.
    fn is_printable(&self, text: &str) -> bool {
        runtime::is_printable(text, self.caps.max_field())
    }

    /// The 400 answer naming the `fields` that failed [`HookState::is_printable`].
    fn unprintable(&self, fields: &str) -> Response {
        (
            StatusCode::BAD_REQUEST,
            format!(
                "{fields} must be 1-{} printable ASCII characters",
                self.caps.max_field()
            ),
        )
            .into_response()
    }

    /// `text` as a fleet id, at most `max_id_len` bytes.
    fn fleet_id(&self, text: &str) -> Option<FleetId> {
        FleetId::parse(text, self.caps.max_id_len())
    }
}

/// `GET /status` query.
#[derive(Debug, Deserialize)]
struct StatusQuery {
    #[serde(default)]
    fleet: Option<String>,
    #[serde(default)]
    session: Option<SessionId>,
    #[serde(default)]
    repo: Option<PathBuf>,
    #[serde(default)]
    worktree: Option<PathBuf>,
}

/// The daemon's routes; the body limit is `hook_max_body` (`PostToolUse` carries the full `tool_response`; axum's
/// default is 2 MB).
pub fn router(state: HookState) -> Router {
    let max_body = state.startup.hook_max_body();
    Router::new()
        .route(HOOK_PATH, post(hook))
        .route("/healthz", get(healthz))
        .route("/status", get(status))
        .route("/subscribe", post(subscribe))
        .route("/unsubscribe", post(unsubscribe))
        .route("/join", post(join))
        .route("/done", post(done))
        .route("/leave", post(leave))
        .route("/remove", post(remove))
        .route("/journal", post(journal))
        .route("/land", post(land))
        .route("/land/end", post(land_end))
        .route("/land/release", post(land_release))
        .route("/worker", post(worker))
        .route("/runtime", get(runtime_status))
        .route("/runtime/subscribe", post(runtime_subscribe))
        .route("/runtime/unsubscribe", post(runtime_unsubscribe))
        .route("/runtime/act", post(runtime_act))
        .layer(DefaultBodyLimit::max(max_body))
        .with_state(state)
}

async fn hook(State(state): State<HookState>, body: Bytes) -> (StatusCode, Json<Value>) {
    let at = Timestamp::now();
    if let Some(recorder) = &state.recorder {
        let _ = recorder.send(Recorded {
            at,
            body: body.to_vec(),
        });
    }
    let mut event = match HookEvent::parse(&body, at) {
        Ok(event) => event,
        Err(err) => {
            log::line(format_args!("POST /hook: unparseable body: {err}"));
            return (
                StatusCode::BAD_REQUEST,
                Json(json!({ "error": err.to_string() })),
            );
        }
    };
    if !state.identity.owns(event.transcript_path.as_deref()) {
        let detail = format!(
            "session {} belongs to another claude home ({})",
            event.session_id,
            event
                .transcript_path
                .as_deref()
                .map_or_else(String::new, |path| path.display().to_string()),
        );
        log::line(format_args!("POST /hook: {detail}"));
        let _ = state.inbox.try_send(sensor(
            SensorName::Hooks,
            SensorState::Degraded,
            Some(detail),
        ));
        return (
            StatusCode::CONFLICT,
            Json(json!({ "error": "foreign claude home" })),
        );
    }
    let _ = state
        .inbox
        .try_send(sensor(SensorName::Hooks, SensorState::Ok, None));
    attach_enforce(&state.identity.claude_home, &mut event);
    event.muted = switches::is_off(&state.identity.claude_home, &event.session_id);
    let stopped = || {
        (
            StatusCode::SERVICE_UNAVAILABLE,
            Json(json!({ "error": "engine stopped" })),
        )
    };
    let (reply, verdict) = oneshot::channel();
    if state
        .inbox
        .send(Input::Hook { event, reply })
        .await
        .is_err()
    {
        return stopped();
    }
    match verdict.await {
        Ok(verdict) => (StatusCode::OK, Json(verdict_body(&verdict))),
        Err(_) => stopped(),
    }
}

/// The `/hook` answer body of a verdict (hooks doc: JSON output, Decision control).
fn verdict_body(verdict: &Verdict) -> Value {
    match verdict {
        Verdict::Allow => json!({}),
        Verdict::Context { event, text } => json!({
            "hookSpecificOutput": {
                "hookEventName": event.hook_event_name(),
                "additionalContext": text,
            },
        }),
        Verdict::Deny { reason } => json!({
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
            },
        }),
        Verdict::Stop {
            reason,
            pre_tool: true,
        } => json!({
            "continue": false,
            "stopReason": reason,
            "systemMessage": reason,
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
            },
        }),
        Verdict::Stop {
            reason,
            pre_tool: false,
        } => json!({
            "continue": false,
            "stopReason": reason,
            "systemMessage": reason,
        }),
        Verdict::Block { reason } => json!({
            "decision": "block",
            "reason": reason,
        }),
    }
}

/// Attaches the deployed enforce declaration when the hook opens a skill run — a root slash prompt or a root `Skill`
/// call (D3). No file: an unenforced run; a file that does not parse is logged and the run stays unenforced.
fn attach_enforce(home: &Path, event: &mut HookEvent) {
    if event.agent.is_some() {
        return;
    }
    let skill = match &event.kind {
        HookKind::UserPromptSubmit { prompt } => slash_skill(prompt),
        HookKind::PreToolUse(call) if call.tool_name == SKILL_TOOL => call
            .input
            .get("skill")
            .and_then(Value::as_str)
            .map(str::to_owned),
        _ => None,
    };
    let Some(skill) = skill else {
        return;
    };
    let path = home.join("skills").join(&skill).join(ENFORCE_FILE);
    let Ok(bytes) = std::fs::read(&path) else {
        return;
    };
    match EnforceDoc::parse(&bytes) {
        Ok(doc) => event.enforce = Some(Arc::new(doc)),
        Err(err) => log::line(format_args!("POST /hook: {}: {err}", path.display())),
    }
}

async fn healthz(State(state): State<HookState>) -> Json<Health> {
    Json(state.identity.health())
}

async fn status(State(state): State<HookState>, Query(query): Query<StatusQuery>) -> Response {
    let fleet = match query.fleet.as_deref().map(|text| state.fleet_id(text)) {
        None => None,
        Some(Some(id)) => Some(id),
        Some(None) => {
            return (
                StatusCode::BAD_REQUEST,
                format!("fleet must be {FLEET_ID_CHARSET}"),
            )
                .into_response();
        }
    };
    let filter = Filter {
        session: query.session,
        repo: query.repo,
        fleet,
        worktree: query.worktree,
    };
    let (reply, answer) = oneshot::channel();
    if state
        .inbox
        .send(Input::Query { filter, reply })
        .await
        .is_err()
    {
        return (StatusCode::SERVICE_UNAVAILABLE, "engine stopped").into_response();
    }
    match answer.await {
        Ok(document) => Json(document).into_response(),
        Err(_) => (StatusCode::SERVICE_UNAVAILABLE, "engine stopped").into_response(),
    }
}

async fn subscribe(State(state): State<HookState>, body: Bytes) -> Response {
    let request = match serde_json::from_slice::<SubscribeBody>(&body) {
        Ok(body) => shape(body, &state.caps),
        Err(err) => Err(format!("subscribe body: {err}")),
    };
    let request = match request {
        Ok(request) => request,
        Err(reason) => return (StatusCode::BAD_REQUEST, reason).into_response(),
    };
    let session = request.session.clone();
    let event = Event::Subscribed {
        request,
        source: SubscriptionSource::Request {
            candidate: InstanceId(uuid::Uuid::now_v7()),
        },
        at: Timestamp::now(),
    };
    let Some(row) = apply_then_subscription(&state, event, session.clone()).await else {
        return engine_stopped();
    };
    let Some(row) = row else {
        log::line(format_args!(
            "POST /subscribe: no subscription row for {session} after the event"
        ));
        return (StatusCode::SERVICE_UNAVAILABLE, "subscription not applied").into_response();
    };
    match row.state {
        SubscriptionStateWire::Active => Json(row).into_response(),
        SubscriptionStateWire::Refused | SubscriptionStateWire::Ended => {
            (StatusCode::CONFLICT, row.detail.unwrap_or_default()).into_response()
        }
    }
}

async fn unsubscribe(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<UnsubscribeBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("unsubscribe body: {err}")).into_response();
        }
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    let session = SessionId(body.session_id);
    let event = Event::UnsubscribeRequested {
        session: session.clone(),
        token: body.token,
        finish: body.finish,
        at: Timestamp::now(),
    };
    let Some(row) = apply_then_subscription(&state, event, session.clone()).await else {
        return engine_stopped();
    };
    let Some(row) = row else {
        return (
            StatusCode::CONFLICT,
            format!("no active subscription for {session}"),
        )
            .into_response();
    };
    match row.state {
        SubscriptionStateWire::Ended => Json(row).into_response(),
        SubscriptionStateWire::Active => (
            StatusCode::CONFLICT,
            "token does not match the subscription",
        )
            .into_response(),
        SubscriptionStateWire::Refused => (
            StatusCode::CONFLICT,
            format!("no active subscription for {session}"),
        )
            .into_response(),
    }
}

fn engine_stopped() -> Response {
    (StatusCode::SERVICE_UNAVAILABLE, "engine stopped").into_response()
}

/// The document for `filter` at the current queue position; `None` once the engine stopped.
async fn query(state: &HookState, filter: Filter) -> Option<StatusDocument> {
    let (reply, answer) = oneshot::channel();
    state
        .inbox
        .send(Input::Query { filter, reply })
        .await
        .ok()?;
    answer.await.ok()
}

/// Sends `event`, then answers the document for `filter` from the queue position right after it; `None` once the
/// engine stopped.
async fn apply_then_query(
    state: &HookState,
    event: Event,
    filter: Filter,
) -> Option<StatusDocument> {
    state.inbox.send(Input::Event(event)).await.ok()?;
    query(state, filter).await
}

/// Sends `event`, then answers `session`'s subscription row from the queue position right after it; `None` once the
/// engine stopped.
async fn apply_then_subscription(
    state: &HookState,
    event: Event,
    session: SessionId,
) -> Option<Option<SubscriptionRow>> {
    state.inbox.send(Input::Event(event)).await.ok()?;
    let (reply, answer) = oneshot::channel();
    state
        .inbox
        .send(Input::SubscriptionOf { session, reply })
        .await
        .ok()?;
    answer.await.ok()
}

/// The session's join record; `None` once the engine stopped.
async fn join_of(state: &HookState, session: &SessionId) -> Option<Option<Join>> {
    let (reply, answer) = oneshot::channel();
    let asked = Input::JoinOf {
        session: session.clone(),
        reply,
    };
    state.inbox.send(asked).await.ok()?;
    answer.await.ok()
}

/// `POST /join`: 200 with the member row once the session is a member of the fleet, 409 with the refusal reason.
async fn join(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<JoinBody>(&body) {
        Ok(body) => body,
        Err(err) => return (StatusCode::BAD_REQUEST, format!("join body: {err}")).into_response(),
    };
    let Some(fleet) = state.fleet_id(&body.fleet) else {
        return (
            StatusCode::BAD_REQUEST,
            format!("fleet must be {FLEET_ID_CHARSET}"),
        )
            .into_response();
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    let session = SessionId(body.session_id);
    let event = Event::Joined {
        session: session.clone(),
        fleet: fleet.clone(),
        token: body.token,
        source: JoinSource::Request {
            candidate: InstanceId(uuid::Uuid::now_v7()),
        },
        at: Timestamp::now(),
    };
    let filter = Filter {
        fleet: Some(fleet.clone()),
        ..Filter::default()
    };
    let Some(document) = apply_then_query(&state, event, filter).await else {
        return engine_stopped();
    };
    let row = document.members.into_iter().find(|row| {
        row.session_id == session.0 && row.via != ViaWire::Worker && row.member.is_some()
    });
    if let Some(row) = row {
        return Json(row).into_response();
    }
    let (reply, answer) = oneshot::channel();
    let asked = state
        .inbox
        .send(Input::JoinRefusal {
            session: session.clone(),
            reply,
        })
        .await;
    let reason = match (asked, answer.await) {
        (Ok(()), Ok(Some(reason))) => reason,
        _ => format!("{session} is no member of fleet {}", fleet.0),
    };
    (StatusCode::CONFLICT, reason).into_response()
}

/// `POST /done`: a joined session reports done with its join token — 200 with the delivered member row, 409 on a
/// token mismatch. A session without a join is accepted for the landing target of its branch — 200 with its landing
/// turn, 200 with `no_target` when its branch has none, 409 with the refusal.
async fn done(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<DoneBody>(&body) {
        Ok(body) => body,
        Err(err) => return (StatusCode::BAD_REQUEST, format!("done body: {err}")).into_response(),
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    let session = SessionId(body.session_id);
    let Some(join) = join_of(&state, &session).await else {
        return engine_stopped();
    };
    let Some(join) = join else {
        let accepted =
            |landing: LandAnswerRow| Json(DoneAnswer::Accepted { landing }).into_response();
        return request_turn(&state, session, LandTarget::OfSession, body.token, accepted).await;
    };
    if join.token != body.token {
        return (StatusCode::CONFLICT, "token does not match the join").into_response();
    }
    let event = Event::DoneReported {
        session: session.clone(),
        token: Some(body.token),
        at: Timestamp::now(),
    };
    let filter = Filter {
        fleet: Some(join.fleet),
        ..Filter::default()
    };
    let Some(document) = apply_then_query(&state, event, filter).await else {
        return engine_stopped();
    };
    let row = document
        .members
        .into_iter()
        .find(|row| row.session_id == session.0 && row.via != ViaWire::Worker);
    match row {
        Some(member) if member.delivered => Json(DoneAnswer::Delivered { member }).into_response(),
        _ => (StatusCode::CONFLICT, "not a joined session").into_response(),
    }
}

/// `POST /leave`: 409 unless the session joined with this token, else 200 with the member row it had once it is no
/// member anymore.
async fn leave(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<LeaveBody>(&body) {
        Ok(body) => body,
        Err(err) => return (StatusCode::BAD_REQUEST, format!("leave body: {err}")).into_response(),
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    let session = SessionId(body.session_id);
    let Some(join) = join_of(&state, &session).await else {
        return engine_stopped();
    };
    let Some(join) = join else {
        return (StatusCode::CONFLICT, "not a joined session").into_response();
    };
    if join.token != body.token {
        return (StatusCode::CONFLICT, "token does not match the join").into_response();
    }
    let filter = Filter {
        fleet: Some(join.fleet.clone()),
        ..Filter::default()
    };
    let Some(document) = query(&state, filter).await else {
        return engine_stopped();
    };
    // A joined session the status holds no member row for (unknown cwd, another repo) still leaves.
    let row = document
        .members
        .into_iter()
        .find(|row| row.session_id == session.0 && row.via != ViaWire::Worker)
        .unwrap_or_else(|| MemberRow {
            fleet: join.fleet.0.clone(),
            session_id: session.0.clone(),
            via: ViaWire::Joined,
            branch: None,
            member: None,
            token: None,
            delivered: false,
        });
    let event = Event::LeaveRequested {
        session: session.clone(),
        token: body.token,
        at: Timestamp::now(),
    };
    if state.inbox.send(Input::Event(event)).await.is_err() {
        return engine_stopped();
    }
    let Some(after) = join_of(&state, &session).await else {
        return engine_stopped();
    };
    match after {
        None => Json(row).into_response(),
        Some(_) => (StatusCode::CONFLICT, "still a member").into_response(),
    }
}

/// `POST /remove`: the operator removes a lettered member; 200 with the member row it had, 409 when the fleet has
/// no live member with that letter (the subscriber has none). Loopback trust, like `/status`.
async fn remove(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<RemoveBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("remove body: {err}")).into_response();
        }
    };
    let Some(fleet) = state
        .fleet_id(&body.fleet)
        .filter(|_| is_member_letter(&body.member))
    else {
        return (
            StatusCode::BAD_REQUEST,
            format!("fleet id and member letter (A-Z, at most {MAX_MEMBER_LETTERS}) required"),
        )
            .into_response();
    };
    let member = MemberId(body.member);
    let filter = Filter {
        fleet: Some(fleet.clone()),
        ..Filter::default()
    };
    let Some(before) = query(&state, filter.clone()).await else {
        return engine_stopped();
    };
    let Some(row) = before
        .members
        .into_iter()
        .find(|row| row.member.as_deref() == Some(member.0.as_str()))
    else {
        return (StatusCode::CONFLICT, "no live member with that letter").into_response();
    };
    let event = Event::RemoveRequested {
        fleet,
        member: member.clone(),
        at: Timestamp::now(),
    };
    let Some(after) = apply_then_query(&state, event, filter).await else {
        return engine_stopped();
    };
    let is_still_member = after
        .members
        .iter()
        .any(|row| row.member.as_deref() == Some(member.0.as_str()));
    if is_still_member {
        return (StatusCode::CONFLICT, "still a member").into_response();
    }
    Json(row).into_response()
}

/// `A` … `ZZZZ`: an upper-case member letter of at most [`MAX_MEMBER_LETTERS`] letters.
fn is_member_letter(text: &str) -> bool {
    !text.is_empty()
        && text.len() <= MAX_MEMBER_LETTERS
        && text.bytes().all(|byte| byte.is_ascii_uppercase())
}

/// `POST /journal`: 200 with the stored row when the caller is its fleet's active subscriber, 409 otherwise.
async fn journal(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<JournalBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("journal body: {err}")).into_response();
        }
    };
    let Some(kind) = JournalKind::parse(&body.kind) else {
        return (
            StatusCode::BAD_REQUEST,
            "kind must be assigned, decided, merged, spawned or note",
        )
            .into_response();
    };
    let is_letter = |text: &String| is_member_letter(text);
    let is_line = |text: &str, max: usize| {
        !text.is_empty() && text.len() <= max && !text.chars().any(char::is_control)
    };
    let max_subject = state.startup.hook_max_subject();
    let max_detail = state.startup.hook_max_detail();
    let is_shaped = state.is_printable(&body.session_id)
        && state.is_printable(&body.token.0)
        && body.members.iter().all(is_letter)
        && is_line(&body.subject, max_subject)
        && body
            .detail
            .as_deref()
            .is_none_or(|detail| is_line(detail, max_detail));
    if !is_shaped {
        return (
            StatusCode::BAD_REQUEST,
            format!(
                "session id and token 1-{} printable ASCII, members as letters, subject 1-{max_subject} and detail \
                 1-{max_detail} characters without control characters",
                state.caps.max_field()
            ),
        )
            .into_response();
    }
    let session = SessionId(body.session_id);
    let filter = Filter {
        session: Some(session.clone()),
        ..Filter::default()
    };
    let Some(document) = query(&state, filter).await else {
        return engine_stopped();
    };
    let subscribed = document.fleets.iter().find(|fleet| {
        fleet
            .subscriptions
            .iter()
            .any(|row| row.session_id == session.0 && row.state == SubscriptionStateWire::Active)
    });
    let Some(fleet) = subscribed.and_then(|fleet| state.fleet_id(&fleet.id)) else {
        return (StatusCode::CONFLICT, "not the fleet's active subscriber").into_response();
    };
    let entry = JournalEntry {
        kind,
        members: body.members.into_iter().map(MemberId).collect(),
        subject: body.subject,
        detail: body.detail,
        by: session.clone(),
        at: Timestamp::now(),
    };
    let want = JournalRow::from(&entry);
    let event = Event::JournalRecorded {
        fleet: fleet.clone(),
        entry,
        token: Some(body.token),
    };
    let filter = Filter {
        fleet: Some(fleet.clone()),
        ..Filter::default()
    };
    let Some(document) = apply_then_query(&state, event, filter).await else {
        return engine_stopped();
    };
    let stored = document
        .fleets
        .into_iter()
        .flat_map(|fleet| fleet.journal)
        .rev()
        .find(|row| *row == want);
    match stored {
        Some(entry) => Json(RecordedRow {
            fleet: fleet.0,
            entry,
        })
        .into_response(),
        None => (StatusCode::CONFLICT, "not the fleet's active subscriber").into_response(),
    }
}

/// Sends `event`, then answers the land answer kept for `caller` from the queue position right after it; `None`
/// once the engine stopped.
async fn apply_then_land(
    state: &HookState,
    event: Event,
    caller: LandCaller,
) -> Option<Option<LandAnswer>> {
    state.inbox.send(Input::Event(event)).await.ok()?;
    let (reply, answer) = oneshot::channel();
    state
        .inbox
        .send(Input::LandAnswerOf { caller, reply })
        .await
        .ok()?;
    answer.await.ok()
}

/// The response of a land read-back: 200 with `body` of the row, 200 with the done answer `no_target`, 409 with the
/// refusal; a missing answer is a refusal. The caller answers an engine that stopped with a 503.
fn land_response(
    answer: Option<LandAnswer>,
    body: impl FnOnce(LandAnswerRow) -> Response,
) -> Response {
    match answer {
        Some(LandAnswer::Reply { target, reply }) => body(LandAnswerRow::new(target, reply)),
        Some(LandAnswer::NoTarget { base }) => Json(DoneAnswer::NoTarget { base }).into_response(),
        Some(LandAnswer::Refused(reason)) => (StatusCode::CONFLICT, reason).into_response(),
        None => (StatusCode::CONFLICT, "the landing call left no answer").into_response(),
    }
}

/// Sends `session`'s turn request for `target` and answers it through `body`.
async fn request_turn(
    state: &HookState,
    session: SessionId,
    target: LandTarget,
    token: MessagingToken,
    body: impl FnOnce(LandAnswerRow) -> Response,
) -> Response {
    let event = Event::LandRequested {
        session: session.clone(),
        target,
        token,
        source: LandSource::Request,
        at: Timestamp::now(),
    };
    let Some(answer) = apply_then_land(state, event, LandCaller::Session(session)).await else {
        return engine_stopped();
    };
    land_response(answer, body)
}

/// `POST /land`: the calling session asks for the landing turn of the named landing target; 200 with the granted or
/// queued row, 409 with the refusal.
async fn land(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<LandBody>(&body) {
        Ok(body) => body,
        Err(err) => return (StatusCode::BAD_REQUEST, format!("land body: {err}")).into_response(),
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    if let Err(reason) = SubscribeRequest::check_target(&body.target, state.caps.max_field()) {
        return (StatusCode::BAD_REQUEST, reason).into_response();
    }
    let session = SessionId(body.session_id);
    let row = |row: LandAnswerRow| Json(row).into_response();
    let target = LandTarget::Named(body.target);
    request_turn(&state, session, target, body.token, row).await
}

/// `POST /land/end`: the calling session ends its landing entry with its token; 200 with the ended row, 409 when
/// it may not end it.
async fn land_end(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<LandEndBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("land end body: {err}")).into_response();
        }
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    let session = SessionId(body.session_id);
    let event = Event::LandEnded {
        end: LandEnd::Typed {
            session: session.clone(),
            token: body.token,
            outcome: body.outcome.into(),
        },
        at: Timestamp::now(),
    };
    let Some(answer) = apply_then_land(&state, event, LandCaller::Session(session)).await else {
        return engine_stopped();
    };
    land_response(answer, |row| Json(row).into_response())
}

/// `POST /land/release`: the operator ends the holder's turn for the landing target in the repository of `cwd`; 200
/// with the released row, 409 without a holder. Loopback trust, like `/remove`.
async fn land_release(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<LandReleaseBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("land release body: {err}")).into_response();
        }
    };
    if let Err(reason) = SubscribeRequest::check_target(&body.target, state.caps.max_field()) {
        return (StatusCode::BAD_REQUEST, reason).into_response();
    }
    let event = Event::LandEnded {
        end: LandEnd::Released {
            cwd: body.cwd.clone(),
            target: body.target,
        },
        at: Timestamp::now(),
    };
    let Some(answer) = apply_then_land(&state, event, LandCaller::Operator(body.cwd)).await else {
        return engine_stopped();
    };
    land_response(answer, |row| Json(row).into_response())
}

/// `POST /worker`: 200 with the worker's member row when the caller is its fleet's active subscriber, 409 otherwise.
/// A worker branch lives in the `worker_branch_prefix` namespace.
async fn worker(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<WorkerBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("worker body: {err}")).into_response();
        }
    };
    let prefix = state.startup.worker_branch_prefix();
    let is_shaped = state.is_printable(&body.session_id)
        && state.is_printable(&body.token.0)
        && state.is_printable(&body.branch)
        && body.branch.len() > prefix.len()
        && body.branch.starts_with(prefix);
    if !is_shaped {
        return (
            StatusCode::BAD_REQUEST,
            format!(
                "session id and token 1-{} printable ASCII, branch {prefix}<name>",
                state.caps.max_field()
            ),
        )
            .into_response();
    }
    let session = SessionId(body.session_id);
    let filter = Filter {
        session: Some(session.clone()),
        ..Filter::default()
    };
    let Some(document) = query(&state, filter).await else {
        return engine_stopped();
    };
    let subscribed = document.fleets.iter().find(|fleet| {
        fleet
            .subscriptions
            .iter()
            .any(|row| row.session_id == session.0 && row.state == SubscriptionStateWire::Active)
    });
    let Some(fleet) = subscribed.and_then(|fleet| state.fleet_id(&fleet.id)) else {
        return (StatusCode::CONFLICT, "not the fleet's active subscriber").into_response();
    };
    let delivered = body.state == WorkerStateWire::Delivered;
    let event = Event::WorkerSet {
        fleet: fleet.clone(),
        branch: WorkerBranch(body.branch.clone()),
        delivered,
        by: session,
        token: Some(body.token),
        at: Timestamp::now(),
    };
    let filter = Filter {
        fleet: Some(fleet),
        ..Filter::default()
    };
    let Some(document) = apply_then_query(&state, event, filter).await else {
        return engine_stopped();
    };
    let stored = document.members.into_iter().find(|row| {
        row.via == ViaWire::Worker
            && row.branch.as_deref() == Some(body.branch.as_str())
            && row.delivered == delivered
    });
    match stored {
        Some(row) => Json(row).into_response(),
        None => (
            StatusCode::CONFLICT,
            "worker not registered: token mismatch or removed",
        )
            .into_response(),
    }
}

/// `GET /runtime`: the runtime document at the current queue position.
async fn runtime_status(State(state): State<HookState>) -> Response {
    let (reply, answer) = oneshot::channel();
    if state
        .inbox
        .send(Input::RuntimeQuery { reply })
        .await
        .is_err()
    {
        return engine_stopped();
    }
    match answer.await {
        Ok(document) => Json(document).into_response(),
        Err(_) => engine_stopped(),
    }
}

/// The session's runtime subscription (or its latest refusal) at the current queue position; `None` once the engine
/// stopped.
async fn runtime_subscription_of(
    state: &HookState,
    session: &SessionId,
) -> Option<SubscriptionAnswer> {
    let (reply, answer) = oneshot::channel();
    let asked = Input::RuntimeSubscriptionOf {
        session: session.clone(),
        reply,
    };
    state.inbox.send(asked).await.ok()?;
    answer.await.ok()
}

/// `POST /runtime/subscribe`: 200 with the subscription row once the session holds it, 409 with the refusal.
async fn runtime_subscribe(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<RuntimeSubscribeBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (
                StatusCode::BAD_REQUEST,
                format!("runtime subscribe body: {err}"),
            )
                .into_response();
        }
    };
    let request = RuntimeSubscribeRequest::parse(
        &body.session_id,
        &body.target,
        body.mode.into(),
        body.socket_path,
        body.token,
        state.caps.max_field(),
    )
    .and_then(|request| is_socket(&request.socket).map(|()| request));
    let request = match request {
        Ok(request) => request,
        Err(reason) => return (StatusCode::BAD_REQUEST, reason).into_response(),
    };
    let session = request.session.clone();
    let event = Event::RuntimeSubscribed {
        request,
        replay: false,
        at: Timestamp::now(),
    };
    if state.inbox.send(Input::Event(event)).await.is_err() {
        return engine_stopped();
    }
    match runtime_subscription_of(&state, &session).await {
        None => engine_stopped(),
        Some(SubscriptionAnswer::Held(row)) => Json(row).into_response(),
        Some(SubscriptionAnswer::Refused(reason)) => (StatusCode::CONFLICT, reason).into_response(),
        Some(SubscriptionAnswer::Absent) => {
            (StatusCode::SERVICE_UNAVAILABLE, "subscription not applied").into_response()
        }
    }
}

/// `POST /runtime/unsubscribe`: 200 with the row the session held once it holds none, 409 otherwise.
async fn runtime_unsubscribe(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<RuntimeUnsubscribeBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (
                StatusCode::BAD_REQUEST,
                format!("runtime unsubscribe body: {err}"),
            )
                .into_response();
        }
    };
    if !state.is_printable(&body.session_id) || !state.is_printable(&body.token.0) {
        return state.unprintable("session id and token");
    }
    let session = SessionId(body.session_id);
    let Some(before) = runtime_subscription_of(&state, &session).await else {
        return engine_stopped();
    };
    let SubscriptionAnswer::Held(row) = before else {
        return (
            StatusCode::CONFLICT,
            format!("no active runtime subscription for {session}"),
        )
            .into_response();
    };
    let event = Event::RuntimeUnsubscribeRequested {
        session: session.clone(),
        token: body.token,
        at: Timestamp::now(),
    };
    if state.inbox.send(Input::Event(event)).await.is_err() {
        return engine_stopped();
    }
    match runtime_subscription_of(&state, &session).await {
        None => engine_stopped(),
        Some(SubscriptionAnswer::Held(_)) => (
            StatusCode::CONFLICT,
            "token does not match the subscription",
        )
            .into_response(),
        Some(SubscriptionAnswer::Refused(_) | SubscriptionAnswer::Absent) => {
            Json(row).into_response()
        }
    }
}

/// `POST /runtime/act`: 200 with the finding row once the move's claim is on disk, 409 with the refusal (or why the
/// move is void).
async fn runtime_act(State(state): State<HookState>, body: Bytes) -> Response {
    let body = match serde_json::from_slice::<RuntimeActBody>(&body) {
        Ok(body) => body,
        Err(err) => {
            return (StatusCode::BAD_REQUEST, format!("runtime act body: {err}")).into_response();
        }
    };
    let is_shaped = state.is_printable(&body.session_id)
        && state.is_printable(&body.token.0)
        && state.is_printable(&body.finding);
    if !is_shaped {
        return state.unprintable("session id, token and finding");
    }
    let finding = FindingId(body.finding);
    let request = ActRequest {
        session: SessionId(body.session_id),
        token: body.token,
        finding: finding.clone(),
        action: body.action.into(),
    };
    let event = Event::RuntimeActRequested {
        request,
        at: Timestamp::now(),
    };
    if state.inbox.send(Input::Event(event)).await.is_err() {
        return engine_stopped();
    }
    // How often, and how many times, a finding whose claim is not on disk yet is re-read.
    for _ in 0..state.startup.claim_polls() {
        let (reply, answer) = oneshot::channel();
        let asked = Input::RuntimeActOf {
            finding: finding.clone(),
            reply,
        };
        if state.inbox.send(asked).await.is_err() {
            return engine_stopped();
        }
        match answer.await {
            Err(_) => return engine_stopped(),
            Ok(ActAnswer::Settled(row)) => return Json(row).into_response(),
            Ok(ActAnswer::Refused(reason) | ActAnswer::Void(reason)) => {
                return (StatusCode::CONFLICT, reason).into_response();
            }
            Ok(ActAnswer::Claiming) => tokio::time::sleep(state.startup.claim_poll()).await,
        }
    }
    (
        StatusCode::SERVICE_UNAVAILABLE,
        "the move's claim is not on disk yet; see runtime status",
    )
        .into_response()
}

/// The edge half of admission: field shapes (the target's branch charset included), then the socket path must name a
/// socket (never a symlink). The repo is not an input: the engine resolves it from the session's cwd.
fn shape(body: SubscribeBody, caps: &Caps) -> Result<SubscribeRequest, String> {
    let SubscribeBody {
        fleet,
        session_id,
        socket_path,
        token,
        target,
        outside_collisions,
    } = body;
    let request = SubscribeRequest::parse(
        &fleet,
        &session_id,
        &target,
        socket_path,
        token,
        outside_collisions,
        caps,
    )?;
    is_socket(&request.socket)?;
    Ok(request)
}

#[cfg(unix)]
fn is_socket(path: &Path) -> Result<(), String> {
    use std::os::unix::fs::FileTypeExt as _;
    match std::fs::symlink_metadata(path) {
        Ok(meta) if meta.file_type().is_socket() => Ok(()),
        Ok(_) => Err(format!("{} is not a socket", path.display())),
        Err(err) => Err(format!("{}: {err}", path.display())),
    }
}

#[cfg(not(unix))]
fn is_socket(_path: &Path) -> Result<(), String> {
    Err("messaging sockets are not supported on windows".to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::attack::ContextEvent;

    #[test]
    fn verdict_bodies() {
        let deny = json!({
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": "r",
        });
        let cases = [
            ("allow", Verdict::Allow, json!({})),
            (
                "deny",
                Verdict::Deny {
                    reason: "r".to_owned(),
                },
                json!({ "hookSpecificOutput": deny.clone() }),
            ),
            (
                "stop-before-tool",
                Verdict::Stop {
                    reason: "r".to_owned(),
                    pre_tool: true,
                },
                json!({ "continue": false, "stopReason": "r", "systemMessage": "r", "hookSpecificOutput": deny }),
            ),
            (
                "stop-after-tool",
                Verdict::Stop {
                    reason: "r".to_owned(),
                    pre_tool: false,
                },
                json!({ "continue": false, "stopReason": "r", "systemMessage": "r" }),
            ),
            (
                "member-context",
                Verdict::Context {
                    event: ContextEvent::UserPromptSubmit,
                    text: "c".to_owned(),
                },
                json!({
                    "hookSpecificOutput": {
                        "hookEventName": "UserPromptSubmit",
                        "additionalContext": "c",
                    }
                }),
            ),
            (
                "runtime-hand-over",
                Verdict::Context {
                    event: ContextEvent::PostToolUse,
                    text: "t".to_owned(),
                },
                json!({
                    "hookSpecificOutput": {
                        "hookEventName": "PostToolUse",
                        "additionalContext": "t",
                    }
                }),
            ),
        ];
        for (label, verdict, want) in cases {
            assert_eq!(verdict_body(&verdict), want, "{label}");
        }
    }
}
