//! The pure transition function: `apply(fleet, event) -> actions`. No clock, no I/O — the same event sequence always
//! yields the same fleet and actions, which the replay tests and the load bench rely on.
//!
//! Session state machine, per event:
//! * `SessionStart` (non-compact) starts a lifecycle → idle, from any state; `compact` keeps the state.
//! * root `UserPromptSubmit` → running and opens a turn; root `PreToolUse` / `PostToolUse(Failure)` → running;
//!   a root `PreToolUse` of `AskUserQuestion` / `ExitPlanMode` → waiting.
//! * root `PermissionRequest` and the root's attention notifications → waiting; a subagent's are its own.
//! * root `Stop` → blocked while a turn denial is pending, else idle; a `Stop` a runtime stop check blocked changes
//!   nothing — the turn goes on.
//! * `SessionEnd` → ended; `StopFailure`, and a process exit without `SessionEnd` → died.
//! * a terminal session ignores everything but a non-compact `SessionStart`.
//!
//! Denials (second precision): blocked means the turn ended with a denial as its last tool outcome — agents route
//! around a denial rather than retry it, so any later completed call (root or subagent, any program) resolves it.
//! One whose second ended by the turn start is an earlier turn's, and one whose second ended by the last completed
//! call arrived late (ledger latency) and is superseded; both are ignored. A pending denial clears on a completed
//! call or a new prompt; one arriving while idle blocks only if its second began by the last `Stop`. For a seeded
//! session no hook touched, a denial older than the registry's `statusUpdatedAt` is replayed history and ignored.
//! Subagent activity (an `agent_id` a `SubagentStart` registered) never moves the root state but resolves denials.
//! An unregistered `agent_id` counts as the root inside an open turn (an `--agent` session) and as a background helper
//! after the root's `Stop`: it then moves neither the state nor the cwd.
//! Registry upserts seed new sessions and never transition existing ones.
//! Git state (P2): hooks, ref changes, scans and plan reads fold through [`crate::repo`].
//! Heart attack: hooks and transcript reads fold through [`crate::attack`] — the reads of unknown subagent models.
//! Runtime enforcement: hooks, the socket edge's and act requests' events fold through [`crate::runtime`] on the
//! runtime moved out beside the fleet; its flush runs after the fleet digest pass.
//!
//! Fleets (P3): a registry upsert sets the session's messaging socket; a subscriber's `Stop` and the peek sensor
//! turning ok ask peek for its spawn tree. Membership: a request-sourced join is admitted and gets the fleet's next
//! member letter, a replayed one is trusted; a worker gets its letter at registration, a replayed one from its
//! `member_assigned` line; a
//! joined session's `done` marks it delivered until its next root prompt; the fleet's active subscriber appends journal
//! entries. A request-sourced subscription is admitted
//! against the fleet and bound to the subscriber's repo, a replayed one is trusted. After every event each active
//! subscription diffs its fleet view into pending items and flushes one digest when its session is idle or blocked
//! and none is in flight; a terminal session or a changed socket ends it; every repo's train bases follow its
//! active subscriptions' targets.
//!
//! Fleet runs: a request that starts a name's run uses the UUID its edge minted (`fleet_started`); a replay without
//! one derives the legacy id. Once the registry seeded, a joined session that is terminal or unknown leaves, and a run
//! with no join and no active subscription dissolves: archived in full (`Action::Dissolve`), then dropped.
//!
//! Landing turns ([`crate::land`]): requests and ends fold through the landing module; once the registry seeded, a
//! pass after every event ends the entry of every terminal or unknown session and hands a freed turn on. A root prompt
//! never touches a landing entry.

use std::collections::{BTreeMap, BTreeSet};
use std::path::Path;

use jiff::Timestamp;
use serde_json::Value;

use crate::action::{Action, Digest};
use crate::action_ledger::Record;
use crate::attack;
use crate::config::Live;
use crate::digest::{self, Item, ItemKey, View};
use crate::event::{
    Agent, Denial, Event, ExitSighting, HookEvent, HookKind, JoinSource, RegistryEntry, ToolCall,
};
use crate::fleet::{
    Fleet, FleetInstance, Join, JournalEntry, LastDigest, LastEvent, LastFacts, Member, MemberKey,
    Outcome, PendingDenial, Program, Requested, RosterEntry, Sensor, SensorName, SensorState,
    Session, SessionState, Subagent, SubscribeRequest, Subscription, SubscriptionSource,
    SubscriptionState, Via, Worker, member_branch,
};
use crate::ids::{
    DigestId, FleetId, InstanceId, MemberId, MessagingToken, Pid, SessionId, WorkerBranch,
};
use crate::land;
use crate::peek::{SpawnTree, Spawned};
use crate::repo::{self, BranchKey, RepoId, ScanState};
use crate::runtime::{self, Runtime};
use crate::wire::{FleetArchive, SubscriptionRow};

use crate::protocol::{
    BUSY_STATUS, COMPACT_SOURCE, EXIT_WITHOUT_END, REGISTRY_WAIT, SKILL_TOOL, WAITING_STATUS,
};

/// Applies one event and returns the actions it asks for.
#[must_use]
#[allow(clippy::too_many_lines)]
pub fn apply(fleet: &mut Fleet, event: Event) -> Vec<Action> {
    fleet.events_applied += 1;
    let at = event.at();
    let before = views(fleet);
    let mut actions = match event {
        Event::Hook(hook) => on_hook_event(fleet, hook),
        Event::RegistryUpserted { entry, at } => apply_registry(fleet, entry, at),
        Event::RegistryRemoved { .. } => Vec::new(),
        Event::ProcessExited { pid, at, sighting } => {
            apply_exit(fleet, pid, at, sighting);
            Vec::new()
        }
        Event::Denied(denial) => on_denial(fleet, denial),
        Event::SensorChanged {
            sensor,
            state,
            detail,
            at,
        } => apply_sensor(fleet, sensor, state, detail, at),
        Event::ConfigChanged { live, .. } => {
            apply_config(fleet, *live);
            Vec::new()
        }
        Event::ReposChanged { repos, at } => repo::on_registry(fleet, at, repos),
        Event::RefsChanged { repo: id, at } => repo::trigger(fleet, at, &id),
        Event::RepoScanned {
            repo: id,
            started_at,
            in_flight,
            outcome,
            at,
        } => repo::on_scanned(fleet, at, &id, started_at, in_flight, outcome),
        Event::PlanParsed {
            repo: id,
            item,
            plan,
            at,
        } => {
            repo::on_plan(fleet, at, &id, item, plan);
            Vec::new()
        }
        Event::PeekObserved { tree, at } => {
            apply_spawn_tree(fleet, tree, at);
            Vec::new()
        }
        Event::Subscribed {
            request,
            source,
            at,
        } => apply_subscribed(fleet, request, source, at),
        Event::Delivered {
            session,
            digest,
            outcome,
            at,
        } => apply_delivered(fleet, &session, digest, outcome, at),
        Event::Joined {
            session,
            fleet: id,
            token,
            source,
            at,
        } => apply_join(fleet, session, id, token, source, at),
        Event::DoneReported { session, token, at } => {
            apply_done(fleet, &session, token.as_ref(), at)
        }
        Event::JournalRecorded {
            fleet: id,
            entry,
            token,
        } => apply_journal(fleet, id, entry, token.as_ref()),
        Event::WorkerSet {
            fleet: id,
            branch,
            delivered,
            by,
            token,
            at,
        } => apply_worker(fleet, &id, branch, delivered, &by, token.as_ref(), at),
        membership @ (Event::UnsubscribeRequested { .. }
        | Event::MemberAssigned { .. }
        | Event::Resumed { .. }
        | Event::FleetStarted { .. }
        | Event::LeaveRequested { .. }
        | Event::Left { .. }
        | Event::RemoveRequested { .. }
        | Event::Removed { .. }
        | Event::Dissolved { .. }
        | Event::RegistrySeeded { .. }) => apply_lifecycle(fleet, membership),
        Event::AgentModelRead {
            session,
            agent,
            model,
            requested,
            ..
        } => {
            attack::on_model_read(fleet, &session, &agent, model, requested);
            Vec::new()
        }
        runtime_event @ (Event::RuntimeSubscribed { .. }
        | Event::RuntimeUnsubscribeRequested { .. }
        | Event::RuntimeActRequested { .. }
        | Event::RuntimeDelivered { .. }
        | Event::ClaimRecorded { .. }
        | Event::ClaimFailed { .. }
        | Event::TemplateHandedOver { .. }) => runtime::split(fleet, |fleet, state| {
            apply_runtime(fleet, state, runtime_event)
        }),
        landing @ (Event::LandRequested { .. } | Event::LandEnded { .. }) => {
            apply_land(fleet, landing)
        }
    };
    actions.extend(after_event(fleet, &before, at));
    actions
}

/// Landing events: a turn request or an end, folded through [`crate::land`].
fn apply_land(fleet: &mut Fleet, event: Event) -> Vec<Action> {
    match event {
        Event::LandRequested {
            session,
            target,
            token,
            source,
            at,
        } => land::on_requested(fleet, session, target, token, source, at),
        Event::LandEnded { end, at } => land::on_ended(fleet, end, at),
        Event::Hook(_)
        | Event::RegistryUpserted { .. }
        | Event::RegistryRemoved { .. }
        | Event::ProcessExited { .. }
        | Event::Denied(_)
        | Event::SensorChanged { .. }
        | Event::RefsChanged { .. }
        | Event::RepoScanned { .. }
        | Event::PlanParsed { .. }
        | Event::PeekObserved { .. }
        | Event::Subscribed { .. }
        | Event::Delivered { .. }
        | Event::Joined { .. }
        | Event::DoneReported { .. }
        | Event::JournalRecorded { .. }
        | Event::UnsubscribeRequested { .. }
        | Event::MemberAssigned { .. }
        | Event::Resumed { .. }
        | Event::FleetStarted { .. }
        | Event::LeaveRequested { .. }
        | Event::Left { .. }
        | Event::RemoveRequested { .. }
        | Event::Removed { .. }
        | Event::Dissolved { .. }
        | Event::RegistrySeeded { .. }
        | Event::AgentModelRead { .. }
        | Event::RuntimeSubscribed { .. }
        | Event::RuntimeUnsubscribeRequested { .. }
        | Event::RuntimeActRequested { .. }
        | Event::RuntimeDelivered { .. }
        | Event::ClaimRecorded { .. }
        | Event::ClaimFailed { .. }
        | Event::TemplateHandedOver { .. }
        | Event::ConfigChanged { .. }
        | Event::ReposChanged { .. }
        | Event::WorkerSet { .. } => Vec::new(),
    }
}

/// The live swap: the config edge's parsed live keys replace the running ones; runs and findings stay where they are.
fn apply_config(fleet: &mut Fleet, live: Live) {
    fleet.exemptions = live.exemptions;
    fleet.lane_prefixes = live.lane_prefixes;
    fleet.forbidden_models = live.forbidden_models;
    fleet.caps = live.caps;
    fleet.runtime.texts = live.texts;
}

/// The passes every event ends with: roster, the fleet digests, the runtime flush, departures, dissolves, landing
/// entries of gone sessions and train bases.
fn after_event(
    fleet: &mut Fleet,
    before: &BTreeMap<SessionId, View>,
    at: Timestamp,
) -> Vec<Action> {
    record_worker_branches(fleet);
    track_roster(fleet);
    let mut actions = observe(fleet, before, at);
    actions.extend(runtime::split(fleet, runtime::flush));
    actions.extend(leave_departed(fleet, at));
    actions.extend(dissolve_empty(fleet, at));
    actions.extend(land::settle(fleet, at));
    actions.extend(sync_train_bases(fleet, at));
    actions
}

/// A hook folds through git state, the heart attack, peek fetches and resumes, then the session state machine. A root
/// `Stop` the runtime blocked — and whose answer a muted session does not swallow — keeps the turn open.
fn on_hook_event(fleet: &mut Fleet, hook: HookEvent) -> Vec<Action> {
    let mut actions = repo::on_hook(fleet, &hook);
    actions.extend(attack::on_hook(fleet, &hook));
    actions.extend(runtime::on_hook_split(fleet, &hook));
    actions.extend(peek_fetches(fleet, &hook));
    actions.extend(resume_on_prompt(fleet, &hook));
    let is_blocked_stop =
        !hook.muted && runtime::is_blocked_root_stop(&fleet.runtime, &hook.session_id);
    apply_hook(fleet, hook, is_blocked_stop);
    actions
}

/// Membership and fleet-run events: an unsubscribe request, a replayed letter, resume, start, leave or dissolve, a
/// leave or remove request, and the registry seed.
fn apply_lifecycle(fleet: &mut Fleet, event: Event) -> Vec<Action> {
    match event {
        Event::UnsubscribeRequested {
            session,
            token,
            finish,
            at,
        } => apply_unsubscribe(fleet, &session, &token, finish, at),
        Event::MemberAssigned {
            fleet: id,
            key,
            member,
            at,
        } => {
            // A worker's letter line is written at registration, so it also brings back a removed worker.
            if let Some(instance) = fleet.instances.get_mut(&id) {
                instance.removed.remove(&member);
            }
            replay_letter(fleet, &id, key, member, at);
            Vec::new()
        }
        Event::Resumed { session, .. } => {
            fleet.delivered.remove(&session);
            Vec::new()
        }
        Event::FleetStarted {
            fleet: name,
            instance,
            at,
        } => {
            fleet
                .instances
                .insert(name.clone(), FleetInstance::new(instance, name, at));
            Vec::new()
        }
        Event::LeaveRequested { session, token, at } => apply_leave(fleet, &session, &token, at),
        Event::Left { session, at } => {
            let _ = leave(fleet, &session, "", at);
            Vec::new()
        }
        Event::RemoveRequested {
            fleet: name,
            member,
            at,
        } => apply_remove(fleet, &name, &member, at, true),
        Event::Removed {
            fleet: name,
            member,
            at,
        } => apply_remove(fleet, &name, &member, at, false),
        Event::Dissolved {
            fleet: name,
            instance,
            ..
        } => {
            if fleet
                .instances
                .get(&name)
                .is_some_and(|live| live.id == instance)
            {
                fleet.instances.remove(&name);
            }
            Vec::new()
        }
        Event::RegistrySeeded { .. } => {
            fleet.seeded = true;
            Vec::new()
        }
        Event::Hook(_)
        | Event::RegistryUpserted { .. }
        | Event::RegistryRemoved { .. }
        | Event::ProcessExited { .. }
        | Event::Denied(_)
        | Event::SensorChanged { .. }
        | Event::RefsChanged { .. }
        | Event::RepoScanned { .. }
        | Event::PlanParsed { .. }
        | Event::PeekObserved { .. }
        | Event::Subscribed { .. }
        | Event::Delivered { .. }
        | Event::Joined { .. }
        | Event::DoneReported { .. }
        | Event::JournalRecorded { .. }
        | Event::WorkerSet { .. }
        | Event::AgentModelRead { .. }
        | Event::RuntimeSubscribed { .. }
        | Event::RuntimeUnsubscribeRequested { .. }
        | Event::RuntimeActRequested { .. }
        | Event::RuntimeDelivered { .. }
        | Event::ClaimRecorded { .. }
        | Event::ClaimFailed { .. }
        | Event::TemplateHandedOver { .. }
        | Event::LandRequested { .. }
        | Event::ConfigChanged { .. }
        | Event::ReposChanged { .. }
        | Event::LandEnded { .. } => Vec::new(),
    }
}

/// Runtime enforcement's events ([`crate::runtime`]), on the runtime moved out beside the fleet.
fn apply_runtime(fleet: &Fleet, state: &mut Runtime, event: Event) -> Vec<Action> {
    match event {
        Event::RuntimeSubscribed {
            request,
            replay,
            at,
        } => runtime::on_subscribed(fleet, state, request, replay, at),
        Event::RuntimeUnsubscribeRequested { session, token, .. } => {
            runtime::on_unsubscribe(state, &session, &token)
        }
        Event::RuntimeActRequested { request, .. } => runtime::on_act(fleet, state, request),
        Event::RuntimeDelivered {
            digest,
            outcome,
            at,
        } => runtime::on_delivered(state, digest, outcome, at),
        Event::ClaimRecorded { finding, .. } => {
            runtime::on_claim_recorded(state, &finding);
            Vec::new()
        }
        Event::ClaimFailed { finding, .. } => {
            runtime::on_claim_failed(state, &finding);
            Vec::new()
        }
        Event::TemplateHandedOver { finding, .. } => runtime::on_handed_over(state, &finding),
        Event::Hook(_)
        | Event::RegistryUpserted { .. }
        | Event::RegistryRemoved { .. }
        | Event::ProcessExited { .. }
        | Event::Denied(_)
        | Event::SensorChanged { .. }
        | Event::RefsChanged { .. }
        | Event::RepoScanned { .. }
        | Event::PlanParsed { .. }
        | Event::PeekObserved { .. }
        | Event::Subscribed { .. }
        | Event::Delivered { .. }
        | Event::UnsubscribeRequested { .. }
        | Event::Joined { .. }
        | Event::MemberAssigned { .. }
        | Event::DoneReported { .. }
        | Event::Resumed { .. }
        | Event::JournalRecorded { .. }
        | Event::WorkerSet { .. }
        | Event::FleetStarted { .. }
        | Event::LeaveRequested { .. }
        | Event::Left { .. }
        | Event::RemoveRequested { .. }
        | Event::Removed { .. }
        | Event::Dissolved { .. }
        | Event::RegistrySeeded { .. }
        | Event::AgentModelRead { .. }
        | Event::LandRequested { .. }
        | Event::ConfigChanged { .. }
        | Event::ReposChanged { .. }
        | Event::LandEnded { .. } => Vec::new(),
    }
}

/// The live run of `name`, created from `candidate` (a request) or the legacy derivation (a replay) when none exists;
/// `true` when it was created.
fn ensure_instance(
    fleet: &mut Fleet,
    name: &FleetId,
    candidate: Option<InstanceId>,
    at: Timestamp,
) -> bool {
    if fleet.instances.contains_key(name) {
        return false;
    }
    let id = candidate.unwrap_or_else(|| InstanceId::legacy(name, at));
    fleet
        .instances
        .insert(name.clone(), FleetInstance::new(id, name.clone(), at));
    true
}

/// The `fleet_started` record of `name`'s live run.
fn started_record(fleet: &Fleet, name: &FleetId) -> Option<Action> {
    let instance = fleet.instances.get(name)?;
    Some(Action::Record(Record::FleetStarted {
        fleet: name.clone(),
        instance: instance.id,
    }))
}

/// Drops a joined session's membership when its token matches the join.
fn apply_leave(
    fleet: &mut Fleet,
    session: &SessionId,
    token: &MessagingToken,
    at: Timestamp,
) -> Vec<Action> {
    let matches = fleet
        .joins
        .get(session)
        .is_some_and(|join| join.token == *token);
    if !matches {
        return Vec::new();
    }
    leave(fleet, session, "left", at)
}

/// Removes letter `member` from `name`'s live run on the operator's request: a joined session leaves, a worker stays
/// out of `members`. `requested` records it; a replay only applies it.
fn apply_remove(
    fleet: &mut Fleet,
    name: &FleetId,
    member: &MemberId,
    at: Timestamp,
    requested: bool,
) -> Vec<Action> {
    let Some(instance) = fleet.instances.get_mut(name) else {
        return Vec::new();
    };
    let Some(key) = instance
        .member_ids
        .by_member
        .iter()
        .find(|(_, letter)| *letter == member)
        .map(|(key, _)| key.clone())
    else {
        return Vec::new();
    };
    instance.removed.insert(member.clone());
    if let Some(entry) = instance.roster.get_mut(&key) {
        entry.left_at = Some(at);
    }
    let is_joined_here = key.1.is_none()
        && fleet
            .joins
            .get(&key.0)
            .is_some_and(|join| join.fleet == *name);
    if is_joined_here {
        // `Removed` replays the whole act, so leave's own `Left` line is not written.
        let _ = leave(fleet, &key.0, "removed by the operator", at);
    }
    if !requested {
        return Vec::new();
    }
    vec![Action::Record(Record::Removed {
        fleet: name.clone(),
        member: member.clone(),
    })]
}

/// Removes `session` from its fleet and records why; the roster keeps it with `left_at`.
fn leave(fleet: &mut Fleet, session: &SessionId, reason: &str, at: Timestamp) -> Vec<Action> {
    let Some(join) = fleet.joins.remove(session) else {
        return Vec::new();
    };
    fleet.delivered.remove(session);
    fleet.join_refusals.remove(session);
    if let Some(entry) = fleet
        .instances
        .get_mut(&join.fleet)
        .and_then(|instance| instance.roster.get_mut(&(session.clone(), None)))
    {
        entry.left_at = Some(at);
    }
    vec![Action::Record(Record::Left {
        session_id: session.clone(),
        reason: reason.to_owned(),
    })]
}

/// Once seeded: every joined session that is terminal, or unknown, leaves.
fn leave_departed(fleet: &mut Fleet, at: Timestamp) -> Vec<Action> {
    if !fleet.seeded {
        return Vec::new();
    }
    let departed: Vec<(SessionId, String)> = fleet
        .joins
        .keys()
        .filter_map(|id| match fleet.sessions.get(id) {
            None => Some((
                id.clone(),
                "session gone while heartbeat was down".to_owned(),
            )),
            Some(session) => {
                terminal_word(&session.state).map(|end| (id.clone(), format!("the session {end}")))
            }
        })
        .collect();
    departed
        .into_iter()
        .flat_map(|(id, reason)| leave(fleet, &id, &reason, at))
        .collect()
}

/// Upserts every live run's current members into its roster (letter, branch and delivery as last seen) and keeps
/// the run's binding facts while it is bound.
fn track_roster(fleet: &mut Fleet) {
    let mut seen: Vec<(FleetId, MemberKey, RosterEntry)> = Vec::new();
    let mut facts: Vec<(FleetId, LastFacts)> = Vec::new();
    for name in fleet.instances.keys() {
        let bound = fleet.binding(name).map(|(repo, _)| repo);
        for member in fleet.members(name) {
            let key = member_key(&member);
            let joined_at = match member.via {
                Via::Joined => fleet.joins.get(&member.session).map(|join| join.since),
                Via::Subscriber => fleet
                    .subscriptions
                    .get(&member.session)
                    .map(|subscription| subscription.since),
                Via::Worker => member.worker.as_ref().and_then(|branch| {
                    let instance = fleet.instances.get(name)?;
                    Some(instance.workers.get(branch)?.since)
                }),
            };
            let branch = bound
                .and_then(|repo| member_branch(fleet, repo, &member))
                .and_then(|key| match key {
                    BranchKey::Branch(name) => Some(name),
                    BranchKey::Detached(_) => None,
                });
            let entry = RosterEntry {
                member: fleet.member_id(name, &key).cloned(),
                via: member.via,
                joined_at: joined_at.unwrap_or(fleet.started_at),
                left_at: None,
                branch,
                delivered: fleet.is_delivered(&member),
            };
            seen.push((name.clone(), key, entry));
        }
        if let Some((repo, target)) = fleet.binding(name) {
            let last = LastFacts {
                repo: Some(repo.clone()),
                target: Some(target.to_owned()),
                merge_branch: fleet.merge_branch(name),
                synced: fleet.synced(name),
            };
            facts.push((name.clone(), last));
        }
    }
    for (name, key, entry) in seen {
        let Some(instance) = fleet.instances.get_mut(&name) else {
            continue;
        };
        match instance.roster.get_mut(&key) {
            Some(known) => {
                known.member = entry.member.or(known.member.take());
                known.via = entry.via;
                known.left_at = None;
                known.branch = entry.branch.or(known.branch.take());
                known.delivered = entry.delivered;
            }
            None => {
                instance.roster.insert(key, entry);
            }
        }
    }
    for (name, last) in facts {
        if let Some(instance) = fleet.instances.get_mut(&name) {
            instance.last = LastFacts {
                merge_branch: last.merge_branch.or(instance.last.merge_branch.take()),
                synced: last.synced.or(instance.last.synced),
                ..last
            };
        }
    }
}

/// Once seeded: every live run with no join and no active subscription dissolves — archived in full, then dropped.
fn dissolve_empty(fleet: &mut Fleet, at: Timestamp) -> Vec<Action> {
    if !fleet.seeded {
        return Vec::new();
    }
    let empty: Vec<FleetId> = fleet
        .instances
        .keys()
        .filter(|name| !fleet.joins.values().any(|join| join.fleet == **name))
        .filter(|name| {
            !fleet.subscriptions.values().any(|subscription| {
                subscription.fleet == **name && subscription.state == SubscriptionState::Active
            })
        })
        .cloned()
        .collect();
    let mut actions = Vec::new();
    for name in empty {
        let Some(mut instance) = fleet.instances.remove(&name) else {
            continue;
        };
        for entry in instance.roster.values_mut() {
            entry.left_at.get_or_insert(at);
        }
        let subscriptions: Vec<SubscriptionRow> = fleet
            .subscriptions
            .iter()
            .filter(|(_, subscription)| subscription.instance == Some(instance.id))
            .map(SubscriptionRow::from)
            .collect();
        let record = Record::Dissolved {
            fleet: name,
            instance: instance.id,
        };
        let archive = FleetArchive::from((&instance, subscriptions, at));
        actions.push(Action::Dissolve {
            archive: Box::new(archive),
            record,
        });
    }
    actions
}

/// The session state machine of one hook. `is_blocked_stop`: the runtime blocked this root `Stop`, so the turn goes on —
/// the state and `stopped_at` stay. Permission requests and waiting notifications move only the root.
fn apply_hook(fleet: &mut Fleet, hook: HookEvent, is_blocked_stop: bool) {
    let HookEvent {
        session_id,
        agent,
        cwd,
        transcript_path: _,
        at,
        kind,
        response: _,
        enforce: _,
        muted: _,
        in_flight: _,
    } = hook;
    let observes_start =
        matches!(&kind, HookKind::SessionStart { source } if source != COMPACT_SOURCE);
    if observes_start {
        fleet.unretire(&session_id);
    }
    let waiting_tools = fleet.caps.waiting_tools();
    let waiting_notifications = fleet.caps.waiting_notifications();
    let session = fleet.session_entry(&session_id, at, !observes_start);
    if session.state.is_terminal() && !observes_start {
        return;
    }
    session.last_event = Some(LastEvent {
        name: kind.name().to_owned(),
        at,
    });
    let is_subagent = agent
        .as_ref()
        .is_some_and(|agent| session.subagents.contains_key(&agent.id));
    // An agent_id no SubagentStart registered is the root itself inside an open turn (an `--agent` session); after the
    // root's Stop it is a background helper and never moves the root.
    let is_stray = agent.is_some() && !is_subagent && !session.is_turn_open();
    let is_root = !is_subagent && !is_stray;
    if cwd.is_some() && is_root {
        session.cwd = cwd;
    }
    match kind {
        HookKind::SessionStart { source } => on_start(session, &source, at),
        HookKind::SessionEnd { reason } => session.transition(SessionState::Ended { reason }, at),
        HookKind::StopFailure { error } => session.transition(
            SessionState::Died {
                cause: format!("StopFailure: {error}"),
            },
            at,
        ),
        HookKind::UserPromptSubmit { prompt } => on_prompt(session, &prompt, at),
        HookKind::PreToolUse(call) => on_pre_tool(session, is_root, call, &waiting_tools, at),
        HookKind::PostToolUse(_) | HookKind::PostToolUseFailure(_) => {
            on_post_tool(session, is_root, at);
        }
        HookKind::PermissionRequest(call) if is_root => {
            session.transition(SessionState::Waiting { on: call.tool_name }, at);
        }
        HookKind::Notification { notification_type } if is_root => {
            on_notification(session, notification_type, &waiting_notifications, at);
        }
        HookKind::Stop if is_blocked_stop => {}
        HookKind::Stop => on_stop(session, at),
        HookKind::SubagentStart => on_subagent_start(session, agent, at),
        HookKind::SubagentStop => on_subagent_stop(session, agent.as_ref()),
        HookKind::PermissionRequest(_) | HookKind::Notification { .. } | HookKind::Other { .. } => {
        }
    }
    fleet.retire(&session_id);
}

fn on_start(session: &mut Session, source: &str, at: Timestamp) {
    if source == COMPACT_SOURCE {
        return;
    }
    session.restart();
    session.transition(SessionState::Idle, at);
}

fn on_prompt(session: &mut Session, prompt: &str, at: Timestamp) {
    session.turn_started = Some(at);
    session.pending_denial = None;
    if let Some(skill) = slash_skill(prompt) {
        session.active_skill = Some(skill);
    }
    session.transition(SessionState::Running, at);
}

/// `waiting_tools`: root tools that wait on a human (`waiting_tools`).
fn on_pre_tool(
    session: &mut Session,
    is_root: bool,
    call: ToolCall,
    waiting_tools: &[String],
    at: Timestamp,
) {
    if !is_root {
        return;
    }
    if call.tool_name == SKILL_TOOL
        && let Some(skill) = call.input.get("skill").and_then(Value::as_str)
    {
        session.active_skill = Some(skill.to_owned());
    }
    let next = if waiting_tools.contains(&call.tool_name) {
        SessionState::Waiting { on: call.tool_name }
    } else {
        SessionState::Running
    };
    session.transition(next, at);
}

fn on_post_tool(session: &mut Session, is_root: bool, at: Timestamp) {
    session.pending_denial = None;
    session.last_tool_at = Some(at);
    if is_root {
        session.transition(SessionState::Running, at);
    }
}

/// `waiting_notifications`: notification types that mean a human must act (hooks doc L2247-2337).
fn on_notification(
    session: &mut Session,
    notification_type: String,
    waiting_notifications: &[String],
    at: Timestamp,
) {
    if waiting_notifications.contains(&notification_type) {
        session.transition(
            SessionState::Waiting {
                on: notification_type,
            },
            at,
        );
    }
}

fn on_stop(session: &mut Session, at: Timestamp) {
    session.stopped_at = Some(at);
    let is_active = matches!(
        session.state,
        SessionState::Running | SessionState::Waiting { .. }
    );
    if !is_active {
        return;
    }
    let next = match &session.pending_denial {
        Some(denial) => SessionState::Blocked {
            denial: denial.clone(),
        },
        None => SessionState::Idle,
    };
    session.transition(next, at);
}

fn on_subagent_start(session: &mut Session, agent: Option<Agent>, at: Timestamp) {
    if let Some(Agent { id, agent_type }) = agent {
        let subagent = Subagent {
            agent_type,
            running: true,
            started_at: at,
            model: None,
            requested: Requested::Unknown,
        };
        session.subagents.insert(id, subagent);
    }
}

fn on_subagent_stop(session: &mut Session, agent: Option<&Agent>) {
    if let Some(agent) = agent
        && let Some(subagent) = session.subagents.get_mut(&agent.id)
    {
        subagent.running = false;
    }
}

fn apply_registry(fleet: &mut Fleet, entry: RegistryEntry, at: Timestamp) -> Vec<Action> {
    let RegistryEntry {
        pid,
        session_id,
        cwd,
        name,
        status,
        waiting_for,
        status_updated_at,
        messaging_socket_path,
    } = entry;
    let previous = fleet.pids.insert(pid, session_id.clone());
    let session = fleet.sessions.entry(session_id.clone()).or_insert_with(|| {
        let mut seeded = Session::new(
            session_id.clone(),
            seed_state(status.as_deref(), waiting_for),
            at,
            true,
        );
        seeded.registry_status_at =
            status_updated_at.and_then(|ms| Timestamp::from_millisecond(ms).ok());
        seeded
    });
    session.pid = Some(pid);
    session.title = name;
    if messaging_socket_path.is_some() {
        session.socket = messaging_socket_path;
    }
    if session.cwd.is_none() {
        session.cwd = Some(cwd);
    }
    if previous.as_ref() == Some(&session_id) {
        Vec::new()
    } else {
        vec![Action::WatchExit { pid }]
    }
}

fn seed_state(status: Option<&str>, waiting_for: Option<String>) -> SessionState {
    match (waiting_for, status) {
        (Some(on), _) => SessionState::Waiting { on },
        (None, Some(WAITING_STATUS)) => SessionState::Waiting {
            on: REGISTRY_WAIT.to_owned(),
        },
        (None, Some(BUSY_STATUS)) => SessionState::Running,
        (None, _) => SessionState::Idle,
    }
}

fn apply_exit(fleet: &mut Fleet, pid: Pid, at: Timestamp, sighting: ExitSighting) {
    let Some(id) = fleet.pids.remove(&pid) else {
        return;
    };
    let Some(session) = fleet.sessions.get_mut(&id) else {
        return;
    };
    if session.pid != Some(pid) {
        return;
    }
    let is_never_observed = sighting == ExitSighting::AlreadyGone
        && session.since_restart
        && session.last_event.is_none();
    if is_never_observed {
        fleet.sessions.remove(&id);
        return;
    }
    if !session.state.is_terminal() {
        session.transition(
            SessionState::Died {
                cause: EXIT_WITHOUT_END.to_owned(),
            },
            at,
        );
    }
    fleet.retire(&id);
}

fn apply_denial(fleet: &mut Fleet, denial: Denial) {
    let Denial {
        session_id,
        tool,
        input,
        candidate_rule,
        at,
    } = denial;
    let is_known = fleet.sessions.contains_key(&session_id);
    if !is_known && at < fleet.started_at {
        return;
    }
    let second_ms = fleet.caps.ledger_second_ms();
    let session = fleet.session_entry(&session_id, at, true);
    if session.state.is_terminal() {
        return;
    }
    let is_earlier_turn = session
        .turn_started
        .is_some_and(|turn| second_ended_by(at, turn, second_ms));
    let is_superseded = session
        .last_tool_at
        .is_some_and(|call| second_ended_by(at, call, second_ms));
    let is_replayed_history = session.last_event.is_none()
        && session
            .registry_status_at
            .is_some_and(|status_at| at < status_at);
    if is_earlier_turn || is_superseded || is_replayed_history {
        return;
    }
    let pending = PendingDenial {
        program: Program::of(&tool, Some(&input)),
        candidate_rule,
        at,
    };
    let precedes_stop = session.stopped_at.is_none_or(|stop| at <= stop);
    if session.state == SessionState::Idle && precedes_stop {
        session.transition(
            SessionState::Blocked {
                denial: pending.clone(),
            },
            at,
        );
    }
    session.pending_denial = Some(pending);
}

/// A denial becomes the session's pending denial.
fn on_denial(fleet: &mut Fleet, denial: Denial) -> Vec<Action> {
    apply_denial(fleet, denial);
    Vec::new()
}

/// The one writer of sensor rows; [`crate::repo`] reports the `agentwt` sensor through it.
pub(crate) fn apply_sensor(
    fleet: &mut Fleet,
    sensor: SensorName,
    state: SensorState,
    detail: Option<String>,
    at: Timestamp,
) -> Vec<Action> {
    let recovered = sensor == SensorName::Peek
        && state == SensorState::Ok
        && fleet
            .sensors
            .get(&sensor)
            .is_none_or(|current| current.state != SensorState::Ok);
    let entry = fleet.sensors.entry(sensor).or_insert(Sensor {
        state,
        detail: None,
        since: at,
    });
    if entry.state != state {
        entry.state = state;
        entry.since = at;
    }
    entry.detail = detail;
    if !recovered {
        return Vec::new();
    }
    active_subscribers(fleet)
        .map(|session| Action::FetchPeek { session })
        .collect()
}

/// The peek call a hook asks for: a subscriber's `Stop` refreshes its spawn tree.
fn peek_fetches(fleet: &Fleet, hook: &HookEvent) -> Vec<Action> {
    match &hook.kind {
        HookKind::Stop if is_active_subscriber(fleet, &hook.session_id) => {
            vec![Action::FetchPeek {
                session: hook.session_id.clone(),
            }]
        }
        HookKind::Stop
        | HookKind::SessionStart { .. }
        | HookKind::SessionEnd { .. }
        | HookKind::UserPromptSubmit { .. }
        | HookKind::PreToolUse(_)
        | HookKind::PostToolUse(_)
        | HookKind::PostToolUseFailure(_)
        | HookKind::PermissionRequest(_)
        | HookKind::Notification { .. }
        | HookKind::StopFailure { .. }
        | HookKind::SubagentStart
        | HookKind::SubagentStop
        | HookKind::Other { .. } => Vec::new(),
    }
}

fn is_active_subscriber(fleet: &Fleet, session: &SessionId) -> bool {
    fleet
        .subscriptions
        .get(session)
        .is_some_and(|subscription| subscription.state == SubscriptionState::Active)
}

fn active_subscribers(fleet: &Fleet) -> impl Iterator<Item = SessionId> + '_ {
    fleet
        .subscriptions
        .iter()
        .filter(|(_, subscription)| subscription.state == SubscriptionState::Active)
        .map(|(id, _)| id.clone())
}

/// A spawn tree adds the subagents hooks did not show (restart rebuild); known subagents keep their hook state.
fn apply_spawn_tree(fleet: &mut Fleet, tree: SpawnTree, at: Timestamp) {
    let SpawnTree { session, subagents } = tree;
    let Some(host) = fleet.sessions.get_mut(&session) else {
        return;
    };
    for Spawned {
        id,
        agent_type,
        at: spawned,
        finished,
    } in subagents
    {
        host.subagents.entry(id).or_insert_with(|| Subagent {
            agent_type: agent_type.unwrap_or_default(),
            running: !finished,
            started_at: spawned.unwrap_or(at),
            model: None,
            requested: Requested::Unknown,
        });
    }
}

/// Admits or refuses a request; replaces the session's earlier subscription and records the change.
fn apply_subscribed(
    fleet: &mut Fleet,
    request: SubscribeRequest,
    source: SubscriptionSource,
    at: Timestamp,
) -> Vec<Action> {
    let SubscribeRequest {
        session,
        fleet: fleet_id,
        target,
        socket,
        token,
        outside_collisions,
    } = request;
    let (admitted, announced, candidate) = match source {
        SubscriptionSource::Request { candidate } => (
            admit(fleet, &session, &fleet_id, &target, &socket),
            BTreeSet::new(),
            Some(candidate),
        ),
        SubscriptionSource::Replay { repo, announced } => (Ok(repo), announced, None),
    };
    let requested = candidate.is_some();
    let (state, repo) = match admitted {
        Ok(repo) => (SubscriptionState::Active, Some(repo)),
        Err(reason) => (SubscriptionState::Refused(reason), None),
    };
    let previous = fleet.subscriptions.get(&session);
    let was_active = previous.is_some_and(|current| current.state == SubscriptionState::Active);
    let identical = previous.is_some_and(|current| {
        was_active
            && repo.is_some()
            && current.fleet == fleet_id
            && current.repo == repo
            && current.target == target
            && current.socket == socket
            && current.token == token
            && current.outside_collisions == outside_collisions
    });
    if identical {
        return Vec::new();
    }
    let mut actions = Vec::new();
    let instance = if repo.is_some() {
        let started = ensure_instance(fleet, &fleet_id, candidate, at);
        if started && requested {
            actions.extend(started_record(fleet, &fleet_id));
        }
        fleet.instances.get(&fleet_id).map(|instance| instance.id)
    } else {
        None
    };
    if requested && let Some(bound) = &repo {
        actions.push(Action::Record(Record::Subscribed {
            session_id: session.clone(),
            fleet: fleet_id.clone(),
            repo: bound.0.clone(),
            target: target.clone(),
            socket_path: socket.clone(),
            token: token.clone(),
            outside_collisions,
        }));
    }
    if repo.is_some() {
        actions.extend(leave_own_join(fleet, &session, &fleet_id, at));
    }
    if requested
        && was_active
        && let SubscriptionState::Refused(reason) = &state
    {
        actions.push(Action::Record(Record::Unsubscribed {
            session_id: session.clone(),
            reason: format!("replaced by a refused subscribe: {reason}"),
        }));
    }
    if repo.is_some() {
        actions.push(Action::FetchPeek {
            session: session.clone(),
        });
    }
    fleet.subscriptions.insert(
        session,
        Subscription {
            fleet: fleet_id,
            instance,
            repo,
            target,
            socket,
            token,
            state,
            since: at,
            pending: BTreeMap::new(),
            in_flight: None,
            last: None,
            announced,
            worker_branches_seen: BTreeSet::new(),
            outside_collisions,
        },
    );
    actions
}

/// An admitted subscriber is its fleet's operator, never also a joined member of it: its own join of that fleet
/// leaves.
fn leave_own_join(
    fleet: &mut Fleet,
    session: &SessionId,
    fleet_id: &FleetId,
    at: Timestamp,
) -> Vec<Action> {
    let is_joined_here = fleet
        .joins
        .get(session)
        .is_some_and(|join| join.fleet == *fleet_id);
    if !is_joined_here {
        return Vec::new();
    }
    leave(fleet, session, "subscribed as the fleet's operator", at)
}

/// Adds every worker's member branch that has a scanned row to its fleet's active subscription (the merge branch
/// excluded): the evidence `Fleet::target_state` needs before `integrated`.
fn record_worker_branches(fleet: &mut Fleet) {
    let seen = worker_branches(fleet);
    for (id, key) in seen {
        if let Some(subscription) = fleet.subscriptions.get_mut(&id) {
            subscription.worker_branches_seen.insert(key);
        }
    }
}

/// Per active subscription, its workers' member branches that have a scanned row, the merge branch and the target
/// excluded.
fn worker_branches(fleet: &Fleet) -> Vec<(SessionId, BranchKey)> {
    fleet
        .subscriptions
        .iter()
        .filter(|(_, subscription)| subscription.state == SubscriptionState::Active)
        .filter_map(|(id, subscription)| Some((id, subscription, subscription.repo.as_ref()?)))
        .flat_map(|(id, subscription, repo)| {
            let skipped: Vec<BranchKey> = fleet
                .merge_branch(&subscription.fleet)
                .into_iter()
                .chain([subscription.target.clone()])
                .map(BranchKey::Branch)
                .collect();
            let rows = fleet.repos.get(repo).map(|state| &state.branches);
            fleet
                .members(&subscription.fleet)
                .into_iter()
                .filter(|member| member.via != Via::Subscriber)
                .filter_map(move |member| member_branch(fleet, repo, &member))
                .filter(move |key| {
                    !skipped.contains(key) && rows.is_some_and(|rows| rows.contains_key(key))
                })
                .map(move |key| (id.clone(), key))
        })
        .collect()
}

/// The engine half of subscribe admission (docs/heartbeat.html § Subscribe): the session is known, alive and owns the
/// socket, its cwd lies in a registered repo, and no other active subscription binds the fleet elsewhere. `Ok` is the
/// fleet's repo; `Err` the refusal reason.
fn admit(
    fleet: &Fleet,
    session: &SessionId,
    fleet_id: &FleetId,
    target: &str,
    socket: &Path,
) -> Result<RepoId, String> {
    let Some(known) = fleet.sessions.get(session) else {
        return Err("unknown session: no hook or registry entry seen for it".to_owned());
    };
    if let Some(end) = terminal_word(&known.state) {
        return Err(format!("the session {end}"));
    }
    if known
        .socket
        .as_deref()
        .is_some_and(|registered| registered != socket)
    {
        return Err(format!(
            "{} is not the session's registered socket",
            socket.display()
        ));
    }
    let cwd = known.cwd.as_deref();
    let Some(repo) = cwd.and_then(|cwd| repo::repo_of(fleet, cwd)) else {
        let shown = cwd.map_or_else(|| "(unknown)".to_owned(), |cwd| cwd.display().to_string());
        return Err(format!("session cwd {shown} is in no registered repo"));
    };
    let elsewhere = fleet
        .subscriptions
        .iter()
        .filter(|(id, subscription)| {
            *id != session
                && subscription.fleet == *fleet_id
                && subscription.state == SubscriptionState::Active
        })
        .map(|(_, subscription)| subscription)
        .find(|subscription| {
            subscription.repo.as_ref() != Some(&repo) || subscription.target != target
        });
    if let Some(bound) = elsewhere {
        let shown = bound
            .repo
            .as_ref()
            .map_or_else(String::new, |bound| bound.0.display().to_string());
        return Err(format!(
            "fleet {} is bound to {shown} {}",
            fleet_id.0, bound.target
        ));
    }
    Ok(repo)
}

/// Keeps every repo's train bases and tracked candidates equal to its bound fleets' ([`Fleet::train_bases`],
/// [`Fleet::candidates`]); a repo whose sets changed is rescanned through P2's coalescing trigger.
fn sync_train_bases(fleet: &mut Fleet, at: Timestamp) -> Vec<Action> {
    let changed: Vec<(RepoId, RepoRequest)> = fleet
        .repos
        .iter()
        .map(|(id, repo)| {
            let request = RepoRequest {
                bases: fleet.train_bases(id),
                candidates: fleet.candidates(id),
                pins: fleet.pins(id),
            };
            (id, request, repo)
        })
        .filter(|(_, request, repo)| {
            request.bases != repo.train_bases
                || request.candidates != repo.candidates
                || request.pins != repo.pins
        })
        .map(|(id, request, _)| (id.clone(), request))
        .collect();
    let mut actions = Vec::new();
    for (id, request) in changed {
        if let Some(repo) = fleet.repos.get_mut(&id) {
            repo.train_bases = request.bases;
            repo.candidates = request.candidates;
            repo.pins = request.pins;
            repo.is_request_changed = repo.scan == ScanState::Running;
        }
        actions.extend(repo::trigger(fleet, at, &id));
    }
    actions
}

/// What a repo's scans are requested for: its train bases, containment candidates and member-branch pins.
struct RepoRequest {
    bases: Vec<String>,
    candidates: Vec<String>,
    pins: BTreeMap<String, String>,
}

/// `ended` or `died` for a terminal state, else `None`: the refusal word for a session that cannot act any more.
pub(crate) fn terminal_word(state: &SessionState) -> Option<&'static str> {
    match state {
        SessionState::Ended { .. } => Some("ended"),
        SessionState::Died { .. } => Some("died"),
        SessionState::Running
        | SessionState::Waiting { .. }
        | SessionState::Blocked { .. }
        | SessionState::Idle => None,
    }
}

/// Settles the in-flight digest; a replayed outcome only fills `last`. `Refused` ends the subscription.
fn apply_delivered(
    fleet: &mut Fleet,
    session: &SessionId,
    digest: DigestId,
    outcome: Outcome,
    at: Timestamp,
) -> Vec<Action> {
    fleet.next_digest = fleet.next_digest.max(DigestId(digest.0 + 1));
    let Some(subscription) = fleet.subscriptions.get_mut(session) else {
        return Vec::new();
    };
    let answered = subscription.in_flight == Some(digest);
    if !answered && subscription.in_flight.is_some() {
        return Vec::new();
    }
    subscription.in_flight = None;
    let refused = match &outcome {
        Outcome::Refused(detail) => answered.then(|| detail.clone()),
        Outcome::Sent | Outcome::Unconfirmed(_) | Outcome::Unclaimed(_) => None,
    };
    subscription.last = Some(LastDigest {
        id: digest,
        outcome,
        at,
    });
    match refused {
        Some(detail) => end_subscription(
            subscription,
            session,
            format!("socket refused: {detail}"),
            at,
        ),
        None => Vec::new(),
    }
}

/// Ends the session's active subscription when the token matches; with `finish` every joined member of the fleet
/// leaves too, so the run dissolves. Anything else changes nothing.
fn apply_unsubscribe(
    fleet: &mut Fleet,
    session: &SessionId,
    token: &MessagingToken,
    finish: bool,
    at: Timestamp,
) -> Vec<Action> {
    let Some(subscription) = fleet.subscriptions.get_mut(session) else {
        return Vec::new();
    };
    if subscription.state != SubscriptionState::Active || subscription.token != *token {
        return Vec::new();
    }
    let name = subscription.fleet.clone();
    let mut actions = end_subscription(
        subscription,
        session,
        "unsubscribed by the session".to_owned(),
        at,
    );
    if !finish {
        return actions;
    }
    let members: Vec<SessionId> = fleet
        .joins
        .iter()
        .filter(|(_, join)| join.fleet == name)
        .map(|(id, _)| id.clone())
        .collect();
    for member in members {
        actions.extend(leave(fleet, &member, "the fleet finished", at));
    }
    actions
}

/// Admits or replays a join. A request is refused for an unknown or ended session and for a fleet bound to another
/// repository than the session's cwd; re-joining keeps the letter, joining another fleet replaces the record.
fn apply_join(
    fleet: &mut Fleet,
    session: SessionId,
    id: FleetId,
    token: MessagingToken,
    source: JoinSource,
    at: Timestamp,
) -> Vec<Action> {
    let key: MemberKey = (session.clone(), None);
    let mut actions = Vec::new();
    let member = match source {
        JoinSource::Replay { member } => {
            close_other_join(fleet, &session, &id, at);
            if let Some(instance) = fleet.instances.get_mut(&id) {
                instance.removed.remove(&member);
            }
            replay_letter(fleet, &id, key, member, at);
            let join = Join {
                fleet: id,
                token,
                since: at,
            };
            fleet.joins.insert(session, join);
            return Vec::new();
        }
        JoinSource::Request { candidate } => {
            if let Err(reason) = admit_join(fleet, &session, &id) {
                fleet.join_refusals.insert(session, (at, reason));
                return Vec::new();
            }
            close_other_join(fleet, &session, &id, at);
            if ensure_instance(fleet, &id, Some(candidate), at) {
                actions.extend(started_record(fleet, &id));
            }
            let Some(instance) = fleet.instances.get_mut(&id) else {
                return actions;
            };
            let member = instance.member_ids.assign(&key).0;
            instance.removed.remove(&member);
            roster_join(instance, key, member.clone(), at);
            member
        }
    };
    fleet.join_refusals.remove(&session);
    let join = Join {
        fleet: id.clone(),
        token: token.clone(),
        since: at,
    };
    fleet.joins.insert(session.clone(), join);
    actions.push(Action::Record(Record::Joined {
        session_id: session,
        fleet: id,
        member,
        token,
    }));
    actions
}

/// Restores a recorded letter into `id`'s live run (a legacy run when none is live) and its roster.
fn replay_letter(fleet: &mut Fleet, id: &FleetId, key: MemberKey, member: MemberId, at: Timestamp) {
    ensure_instance(fleet, id, None, at);
    if let Some(instance) = fleet.instances.get_mut(id) {
        instance
            .member_ids
            .by_member
            .insert(key.clone(), member.clone());
        roster_join(instance, key, member, at);
    }
}

/// Joining another fleet leaves the old one: its roster entry closes at `at` (the new `joined` line replaces the old
/// join on replay, so no `left` line is written).
fn close_other_join(fleet: &mut Fleet, session: &SessionId, id: &FleetId, at: Timestamp) {
    let Some(old) = fleet
        .joins
        .get(session)
        .filter(|join| join.fleet != *id)
        .map(|join| join.fleet.clone())
    else {
        return;
    };
    if let Some(entry) = fleet
        .instances
        .get_mut(&old)
        .and_then(|instance| instance.roster.get_mut(&(session.clone(), None)))
    {
        entry.left_at = Some(at);
    }
}

/// Adds a lettered member to the run's roster when it is not there yet; a returning member is open again.
fn roster_join(instance: &mut FleetInstance, key: MemberKey, member: MemberId, at: Timestamp) {
    let via = if key.1.is_some() {
        Via::Worker
    } else {
        Via::Joined
    };
    let entry = instance.roster.entry(key).or_insert(RosterEntry {
        member: Some(member),
        via,
        joined_at: at,
        left_at: None,
        branch: None,
        delivered: false,
    });
    entry.left_at = None;
}

/// The engine half of join admission.
fn admit_join(fleet: &Fleet, session: &SessionId, id: &FleetId) -> Result<(), String> {
    let Some(known) = fleet.sessions.get(session) else {
        return Err("unknown session: no hook or registry entry seen for it".to_owned());
    };
    if let Some(end) = terminal_word(&known.state) {
        return Err(format!("the session {end}"));
    }
    let Some((bound, _)) = fleet.binding(id) else {
        return Ok(());
    };
    let own = known
        .cwd
        .as_deref()
        .and_then(|cwd| repo::repo_of(fleet, cwd));
    if own.as_ref() == Some(bound) {
        Ok(())
    } else {
        Err(format!("fleet {} is bound to {}", id.0, bound.0.display()))
    }
}

/// Records a joined session as delivered when its token matches; replay trusts the record.
fn apply_done(
    fleet: &mut Fleet,
    session: &SessionId,
    token: Option<&MessagingToken>,
    at: Timestamp,
) -> Vec<Action> {
    let Some(join) = fleet.joins.get(session) else {
        return Vec::new();
    };
    let is_replay = token.is_none();
    if !is_replay && token != Some(&join.token) {
        return Vec::new();
    }
    let is_fresh = fleet.delivered.insert(session.clone(), at).is_none();
    if is_replay || !is_fresh {
        return Vec::new();
    }
    vec![Action::Record(Record::Delivered {
        session_id: session.clone(),
    })]
}

/// A delivered session's next root prompt makes it working again.
fn resume_on_prompt(fleet: &mut Fleet, hook: &HookEvent) -> Vec<Action> {
    let is_prompt = matches!(hook.kind, HookKind::UserPromptSubmit { .. });
    let is_subagent = hook.agent.as_ref().is_some_and(|agent| {
        fleet
            .sessions
            .get(&hook.session_id)
            .is_some_and(|session| session.subagents.contains_key(&agent.id))
    });
    if !is_prompt || is_subagent || fleet.delivered.remove(&hook.session_id).is_none() {
        return Vec::new();
    }
    vec![Action::Record(Record::Resumed {
        session_id: hook.session_id.clone(),
    })]
}

/// Appends an orchestrator's journal entry to the fleet's live run when it comes from the fleet's active subscriber
/// with its token; a replayed entry lands in the live run, a legacy one when none is live.
fn apply_journal(
    fleet: &mut Fleet,
    id: FleetId,
    entry: JournalEntry,
    token: Option<&MessagingToken>,
) -> Vec<Action> {
    let is_replay = token.is_none();
    if !is_replay {
        let is_allowed = fleet
            .subscriptions
            .get(&entry.by)
            .is_some_and(|subscription| {
                subscription.state == SubscriptionState::Active
                    && subscription.fleet == id
                    && Some(&subscription.token) == token
            });
        if !is_allowed {
            return Vec::new();
        }
    }
    ensure_instance(fleet, &id, None, entry.at);
    if let Some(instance) = fleet.instances.get_mut(&id) {
        instance.journal.push(entry.clone());
    }
    if is_replay {
        return Vec::new();
    }
    vec![Action::Record(Record::Journal { fleet: id, entry })]
}

/// Registers a worker branch in the fleet's live run, or changes its delivery, when the call comes from the fleet's
/// active subscriber with its token; a replayed record is trusted. A worker gets its letter at registration, like a
/// joined member at join: a new branch is lettered and its `member_assigned` line follows the `worker` line, and a
/// branch the operator removed is a member again under its old letter. A replay assigns nothing — the replayed
/// `member_assigned` line that follows restores the letter.
fn apply_worker(
    fleet: &mut Fleet,
    id: &FleetId,
    branch: WorkerBranch,
    delivered: bool,
    by: &SessionId,
    token: Option<&MessagingToken>,
    at: Timestamp,
) -> Vec<Action> {
    let is_replay = token.is_none();
    if !is_replay {
        let is_allowed = fleet.subscriptions.get(by).is_some_and(|subscription| {
            subscription.state == SubscriptionState::Active
                && subscription.fleet == *id
                && Some(&subscription.token) == token
        });
        if !is_allowed {
            return Vec::new();
        }
    }
    ensure_instance(fleet, id, None, at);
    let Some(instance) = fleet.instances.get_mut(id) else {
        return Vec::new();
    };
    let worker = instance.workers.entry(branch.clone()).or_insert(Worker {
        by: by.clone(),
        since: at,
        delivered,
    });
    worker.delivered = delivered;
    let key: MemberKey = (worker.by.clone(), Some(branch.clone()));
    if is_replay {
        return Vec::new();
    }
    let (member, is_new) = instance.member_ids.assign(&key);
    let was_removed = instance.removed.remove(&member);
    let mut actions = vec![Action::Record(Record::Worker {
        fleet: id.clone(),
        session_id: by.clone(),
        branch,
        delivered,
    })];
    if is_new || was_removed {
        actions.push(Action::Record(Record::MemberAssigned {
            fleet: id.clone(),
            session_id: key.0,
            worker: key.1,
            agent_id: None,
            member,
        }));
    }
    actions
}

fn member_key(member: &Member) -> MemberKey {
    (member.session.clone(), member.worker.clone())
}

fn end_subscription(
    subscription: &mut Subscription,
    session: &SessionId,
    reason: String,
    at: Timestamp,
) -> Vec<Action> {
    subscription.state = SubscriptionState::Ended(reason.clone());
    subscription.since = at;
    subscription.pending.clear();
    vec![Action::Record(Record::Unsubscribed {
        session_id: session.clone(),
        reason,
    })]
}

/// Every active subscription's view, by subscriber.
fn views(fleet: &Fleet) -> BTreeMap<SessionId, View> {
    fleet
        .subscriptions
        .iter()
        .filter(|(_, subscription)| subscription.state == SubscriptionState::Active)
        .map(|(id, subscription)| (id.clone(), digest::view(fleet, id, subscription)))
        .collect()
}

/// The digest pass: diff each active subscription's view into pending items, end the ones whose session is gone or
/// moved sockets, flush a digest where the subscriber is idle or blocked with nothing in flight, and drop ended or
/// refused subscriptions once P1 evicted their session — except the newest `max_refused_unknown` refusals of an
/// unknown session, which leave only when newer ones push them out (D16: the `/subscribe` handler's query must still
/// find its 409 row after events other edges enqueued between its event and its query).
fn observe(fleet: &mut Fleet, before: &BTreeMap<SessionId, View>, at: Timestamp) -> Vec<Action> {
    let after = views(fleet);
    let mut actions = Vec::new();
    for (id, view) in &after {
        let host = fleet
            .sessions
            .get(id)
            .map(|session| (session.state.clone(), session.socket.clone()));
        let next = fleet.next_digest;
        let Some(subscription) = fleet.subscriptions.get_mut(id) else {
            continue;
        };
        if let Some(was) = before.get(id) {
            for key in digest::gone(was, view) {
                subscription.announced.remove(&key);
            }
            subscription
                .pending
                .extend(digest::diff(was, view, &subscription.announced));
        }
        let Some((state, socket)) = host else {
            continue;
        };
        if let Some(end) = terminal_word(&state) {
            actions.extend(end_subscription(
                subscription,
                id,
                format!("the session {end}"),
                at,
            ));
            continue;
        }
        if socket
            .as_ref()
            .is_some_and(|registered| *registered != subscription.socket)
        {
            actions.extend(end_subscription(
                subscription,
                id,
                "the session's socket changed; subscribe again".to_owned(),
                at,
            ));
            continue;
        }
        let ready = matches!(state, SessionState::Idle | SessionState::Blocked { .. })
            && subscription.in_flight.is_none();
        if !ready {
            continue;
        }
        subscription
            .pending
            .retain(|key, _| digest::holds(key, view));
        if subscription.pending.is_empty() {
            continue;
        }
        let pending = std::mem::take(&mut subscription.pending);
        let keys: Vec<String> = pending
            .keys()
            .filter_map(|key| match key {
                ItemKey::Fleet(key) => Some(key.clone()),
                ItemKey::State(_)
                | ItemKey::WorkItem(_)
                | ItemKey::MergeReady(_)
                | ItemKey::Delivered(_) => None,
            })
            .collect();
        subscription.announced.extend(keys.iter().cloned());
        let items: Vec<&Item> = pending.values().collect();
        subscription.in_flight = Some(next);
        actions.push(Action::DeliverDigest(Digest {
            id: next,
            session: id.clone(),
            fleet: subscription.fleet.clone(),
            socket: subscription.socket.clone(),
            token: subscription.token.clone(),
            text: digest::render(next, &subscription.fleet, &items, &fleet.caps),
            items: u32::try_from(items.len()).unwrap_or(u32::MAX),
            keys,
        }));
        fleet.next_digest = DigestId(next.0 + 1);
    }
    cap_unknown_refusals(fleet);
    actions
}

/// Keeps the newest `max_refused_unknown` subscription and join refusals of sessions heartbeat never saw; refusals
/// of known sessions leave with the session.
fn cap_unknown_refusals(fleet: &mut Fleet) {
    let max_refused_unknown = fleet.caps.max_refused_unknown();
    let sessions = &fleet.sessions;
    let mut unknown_refusals: Vec<(Timestamp, SessionId)> = fleet
        .subscriptions
        .iter()
        .filter(|(id, subscription)| {
            matches!(subscription.state, SubscriptionState::Refused(_))
                && !sessions.contains_key(*id)
        })
        .map(|(id, subscription)| (subscription.since, id.clone()))
        .collect();
    unknown_refusals.sort_unstable_by(|a, b| b.cmp(a));
    let kept: BTreeSet<SessionId> = unknown_refusals
        .into_iter()
        .take(max_refused_unknown)
        .map(|(_, id)| id)
        .collect();
    fleet.subscriptions.retain(|id, subscription| {
        subscription.state == SubscriptionState::Active
            || sessions.contains_key(id)
            || kept.contains(id)
    });
    let mut unknown_joins: Vec<(Timestamp, SessionId)> = fleet
        .join_refusals
        .iter()
        .filter(|(id, _)| !sessions.contains_key(*id))
        .map(|(id, (since, _))| (*since, id.clone()))
        .collect();
    unknown_joins.sort_unstable_by(|a, b| b.cmp(a));
    let dropped: Vec<SessionId> = unknown_joins
        .into_iter()
        .skip(max_refused_unknown)
        .map(|(_, id)| id)
        .collect();
    for id in dropped {
        fleet.join_refusals.remove(&id);
    }
}

/// Whether the ledger second (`second_ms`, the `ts` granularity) starting at `denied` had ended by `moment`.
fn second_ended_by(denied: Timestamp, moment: Timestamp, second_ms: i64) -> bool {
    denied.as_millisecond().saturating_add(second_ms) <= moment.as_millisecond()
}

/// `/name args` → `name`; anything else → `None`.
pub(crate) fn slash_skill(prompt: &str) -> Option<String> {
    let rest = prompt.strip_prefix('/')?;
    let name = rest.split(char::is_whitespace).next()?;
    let is_name = !name.is_empty()
        && name
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '-' | '_' | ':'));
    is_name.then(|| name.to_owned())
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use serde_json::json;

    use super::*;
    use crate::ids::{AgentId, SessionId};
    use crate::wire::SessionStateWire;

    const S: &str = "s";

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    fn hook(second: i64, kind: HookKind) -> Event {
        Event::Hook(HookEvent {
            session_id: SessionId(S.to_owned()),
            agent: None,
            cwd: None,
            transcript_path: None,
            at: at(second),
            kind,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        })
    }

    fn agent_hook(agent: &str, second: i64, kind: HookKind) -> Event {
        let Event::Hook(mut event) = hook(second, kind) else {
            unreachable!("hook() builds Event::Hook")
        };
        event.agent = Some(Agent {
            id: AgentId(agent.to_owned()),
            agent_type: "Explore".to_owned(),
        });
        Event::Hook(event)
    }

    fn start(second: i64) -> Event {
        hook(
            second,
            HookKind::SessionStart {
                source: "startup".to_owned(),
            },
        )
    }

    fn prompt(second: i64, text: &str) -> Event {
        hook(
            second,
            HookKind::UserPromptSubmit {
                prompt: text.to_owned(),
            },
        )
    }

    fn call(tool: &str, input: Value) -> ToolCall {
        ToolCall {
            tool_name: tool.to_owned(),
            input,
        }
    }

    fn bash(command: &str) -> ToolCall {
        call("Bash", json!({ "command": command }))
    }

    fn denial(second: i64, command: &str, rule: &str) -> Event {
        Event::Denied(Denial {
            session_id: SessionId(S.to_owned()),
            tool: "Bash".to_owned(),
            input: command.to_owned(),
            candidate_rule: rule.to_owned(),
            at: at(second),
        })
    }

    fn registry(pid: u32, status: &str) -> Event {
        Event::RegistryUpserted {
            entry: RegistryEntry {
                pid: Pid(pid),
                session_id: SessionId(S.to_owned()),
                cwd: "/repo".into(),
                name: Some("CC,FD - title - [WIP]".to_owned()),
                status: Some(status.to_owned()),
                waiting_for: None,
                status_updated_at: Some(10_000),
                messaging_socket_path: None,
            },
            at: at(1),
        }
    }

    fn exit(pid: u32, second: i64, sighting: ExitSighting) -> Event {
        Event::ProcessExited {
            pid: Pid(pid),
            at: at(second),
            sighting,
        }
    }

    fn replay(events: Vec<Event>) -> Fleet {
        let mut fleet = Fleet::new(at(0));
        for event in events {
            let _ = apply(&mut fleet, event);
        }
        fleet
    }

    fn state(fleet: &Fleet) -> Option<&'static str> {
        fleet
            .sessions
            .get(&SessionId(S.to_owned()))
            .map(|session| SessionStateWire::from(&session.state).as_str())
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn state_machine_replays() {
        let end = |reason: &str| HookKind::SessionEnd {
            reason: reason.to_owned(),
        };
        let note = |kind: &str| HookKind::Notification {
            notification_type: kind.to_owned(),
        };
        let cases: Vec<(&str, Vec<Event>, Option<&str>)> = vec![
            ("start-is-idle", vec![start(1)], Some("idle")),
            (
                "compact-keeps-state",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(
                        3,
                        HookKind::SessionStart {
                            source: "compact".to_owned(),
                        },
                    ),
                ],
                Some("running"),
            ),
            (
                "prompt-runs",
                vec![start(1), prompt(2, "go")],
                Some("running"),
            ),
            (
                "tools-then-stop-is-idle",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::PreToolUse(bash("ls"))),
                    hook(4, HookKind::PostToolUse(bash("ls"))),
                    hook(5, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "stop-with-pending-denial-is-blocked",
                vec![
                    start(1),
                    prompt(2, "go"),
                    denial(3, "git -C /x status", "Bash(git *)"),
                    hook(4, HookKind::Stop),
                ],
                Some("blocked"),
            ),
            (
                "same-program-call-resolves-denial",
                vec![
                    start(1),
                    prompt(2, "go"),
                    denial(3, "git -C /x status", "Bash(git *)"),
                    hook(4, HookKind::PostToolUse(bash("git status"))),
                    hook(5, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "routed-around-denial-resolves",
                vec![
                    start(1),
                    prompt(2, "go"),
                    denial(3, "git -C /x status", "Bash(git *)"),
                    hook(4, HookKind::PostToolUse(bash("ls"))),
                    hook(5, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "failed-call-resolves-denial",
                vec![
                    start(1),
                    prompt(2, "go"),
                    denial(3, "git -C /x status", "Bash(git *)"),
                    hook(4, HookKind::PostToolUseFailure(bash("make audit"))),
                    hook(5, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "denial-as-last-action-blocks",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::PostToolUse(bash("ls"))),
                    denial(4, "git -C /x status", "Bash(git *)"),
                    hook(5, HookKind::Stop),
                ],
                Some("blocked"),
            ),
            (
                "late-ledger-denial-after-a-later-call-is-superseded",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(5, HookKind::PostToolUse(bash("ls"))),
                    denial(3, "git -C /x status", "Bash(git *)"),
                    hook(6, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "prompt-clears-denial",
                vec![
                    start(1),
                    prompt(2, "go"),
                    denial(3, "git -C /x", "Bash(git *)"),
                    hook(4, HookKind::Stop),
                    prompt(5, "use git status"),
                    hook(6, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "earlier-turn-denial-ignored",
                vec![
                    start(1),
                    prompt(10, "go"),
                    denial(5, "ps -o pid", "Bash(ps *)"),
                    hook(11, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "same-second-denial-counts",
                vec![
                    start(1),
                    prompt(10, "go"),
                    denial(10, "ps -o pid", "Bash(ps *)"),
                    hook(11, HookKind::Stop),
                ],
                Some("blocked"),
            ),
            (
                "late-denial-after-stop-is-blocked",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::Stop),
                    denial(3, "ps -o pid", "Bash(ps *)"),
                ],
                Some("blocked"),
            ),
            (
                "denial-after-stop-second-keeps-idle",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::Stop),
                    denial(5, "ps -o pid", "Bash(ps *)"),
                ],
                Some("idle"),
            ),
            (
                "replayed-denial-before-registry-status-ignored",
                vec![registry(100, "idle"), denial(5, "ps", "Bash(ps *)")],
                Some("idle"),
            ),
            (
                "replayed-denial-after-registry-status-blocks",
                vec![registry(100, "idle"), denial(20, "ps", "Bash(ps *)")],
                Some("blocked"),
            ),
            (
                "permission-request-waits",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::PermissionRequest(bash("rm x"))),
                ],
                Some("waiting"),
            ),
            (
                "ask-user-question-waits",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(
                        3,
                        HookKind::PreToolUse(call("AskUserQuestion", Value::Null)),
                    ),
                ],
                Some("waiting"),
            ),
            (
                "answered-question-runs",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(
                        3,
                        HookKind::PreToolUse(call("AskUserQuestion", Value::Null)),
                    ),
                    hook(
                        4,
                        HookKind::PostToolUse(call("AskUserQuestion", Value::Null)),
                    ),
                ],
                Some("running"),
            ),
            (
                "idle-prompt-notification-keeps-idle",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::Stop),
                    hook(4, note("idle_prompt")),
                ],
                Some("idle"),
            ),
            (
                "permission-prompt-notification-waits",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, note("permission_prompt")),
                ],
                Some("waiting"),
            ),
            (
                "subagent-permission-request-keeps-root-running",
                vec![
                    start(1),
                    prompt(2, "go"),
                    agent_hook("a1", 3, HookKind::SubagentStart),
                    agent_hook("a1", 4, HookKind::PermissionRequest(bash("rm x"))),
                ],
                Some("running"),
            ),
            (
                "subagent-notification-keeps-root-running",
                vec![
                    start(1),
                    prompt(2, "go"),
                    agent_hook("a1", 3, HookKind::SubagentStart),
                    agent_hook("a1", 4, note("permission_prompt")),
                ],
                Some("running"),
            ),
            (
                "stop-failure-dies",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(
                        3,
                        HookKind::StopFailure {
                            error: "rate_limit".to_owned(),
                        },
                    ),
                ],
                Some("died"),
            ),
            (
                "session-end-ends-and-ignores-later-events",
                vec![
                    start(1),
                    hook(2, end("other")),
                    hook(3, HookKind::PreToolUse(bash("ls"))),
                ],
                Some("ended"),
            ),
            (
                "session-start-revives-ended",
                vec![
                    start(1),
                    hook(2, end("resume")),
                    hook(
                        3,
                        HookKind::SessionStart {
                            source: "resume".to_owned(),
                        },
                    ),
                ],
                Some("idle"),
            ),
            (
                "exit-without-end-dies",
                vec![registry(100, "busy"), exit(100, 5, ExitSighting::Observed)],
                Some("died"),
            ),
            (
                "exit-after-end-stays-ended",
                vec![
                    registry(100, "busy"),
                    hook(2, end("prompt_input_exit")),
                    exit(100, 3, ExitSighting::Observed),
                ],
                Some("ended"),
            ),
            (
                "stale-registry-file-is-dropped",
                vec![
                    registry(100, "idle"),
                    exit(100, 1, ExitSighting::AlreadyGone),
                ],
                None,
            ),
            (
                "seen-session-already-gone-dies",
                vec![
                    registry(100, "idle"),
                    prompt(2, "go"),
                    exit(100, 3, ExitSighting::AlreadyGone),
                ],
                Some("died"),
            ),
            (
                "registry-seeds-busy-as-running",
                vec![registry(100, "busy")],
                Some("running"),
            ),
            (
                "registry-seeds-waiting",
                vec![registry(100, "waiting")],
                Some("waiting"),
            ),
            (
                "registry-seeds-idle",
                vec![registry(100, "idle")],
                Some("idle"),
            ),
            (
                "registry-never-transitions-a-known-session",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::Stop),
                    registry(100, "busy"),
                ],
                Some("idle"),
            ),
            (
                "subagent-activity-keeps-root-idle",
                vec![
                    start(1),
                    prompt(2, "go"),
                    agent_hook("a1", 3, HookKind::SubagentStart),
                    hook(4, HookKind::Stop),
                    agent_hook("a1", 5, HookKind::PreToolUse(bash("ls"))),
                ],
                Some("idle"),
            ),
            (
                "agent-session-without-subagent-start-is-root",
                vec![
                    start(1),
                    prompt(2, "go"),
                    hook(3, HookKind::Stop),
                    prompt(4, "again"),
                    agent_hook("main", 5, HookKind::PreToolUse(bash("ls"))),
                    hook(6, HookKind::Stop),
                    prompt(7, "more"),
                    agent_hook("main", 8, HookKind::PostToolUse(bash("ls"))),
                ],
                Some("running"),
            ),
            (
                "stray-agent-hook-after-stop-stays-idle",
                vec![
                    start(1),
                    prompt(2, "digest"),
                    hook(3, HookKind::Stop),
                    agent_hook("helper", 4, HookKind::PreToolUse(bash("ls"))),
                    agent_hook("helper", 5, HookKind::PostToolUse(bash("ls"))),
                    agent_hook("helper", 6, HookKind::SubagentStop),
                ],
                Some("idle"),
            ),
            (
                "registered-subagent-hook-after-stop-stays-idle",
                vec![
                    start(1),
                    prompt(2, "go"),
                    agent_hook("a1", 3, HookKind::SubagentStart),
                    hook(4, HookKind::Stop),
                    agent_hook("a1", 5, HookKind::PostToolUse(bash("ls"))),
                ],
                Some("idle"),
            ),
            (
                "prompt-after-stray-hook-runs",
                vec![
                    start(1),
                    prompt(2, "digest"),
                    hook(3, HookKind::Stop),
                    agent_hook("helper", 4, HookKind::PostToolUse(bash("ls"))),
                    prompt(5, "next digest"),
                ],
                Some("running"),
            ),
            (
                "subagent-call-resolves-denial",
                vec![
                    start(1),
                    prompt(2, "go"),
                    agent_hook("a1", 3, HookKind::SubagentStart),
                    denial(4, "git -C /x log", "Bash(git *)"),
                    agent_hook("a1", 5, HookKind::PostToolUse(bash("git log"))),
                    hook(6, HookKind::Stop),
                ],
                Some("idle"),
            ),
            (
                "old-denial-for-unknown-session-is-ignored",
                vec![denial(-10, "ps", "Bash(ps *)")],
                None,
            ),
            (
                "live-denial-creates-the-session",
                vec![denial(5, "ps", "Bash(ps *)")],
                Some("running"),
            ),
            (
                "first-seen-via-stop-is-idle",
                vec![hook(1, HookKind::Stop)],
                Some("idle"),
            ),
        ];
        for (label, events, expected) in cases {
            assert_eq!(state(&replay(events)), expected, "{label}");
        }
    }

    #[test]
    fn since_restart_marks_unobserved_starts() {
        let id = SessionId(S.to_owned());
        let observed = replay(vec![start(1)]);
        let unobserved = replay(vec![prompt(1, "go")]);
        let compacted = replay(vec![hook(
            1,
            HookKind::SessionStart {
                source: "compact".to_owned(),
            },
        )]);
        assert!(!observed.sessions[&id].since_restart);
        assert!(unobserved.sessions[&id].since_restart);
        assert!(compacted.sessions[&id].since_restart);
    }

    #[test]
    fn blocked_carries_the_candidate_rule() {
        let fleet = replay(vec![
            start(1),
            prompt(2, "go"),
            denial(3, "ps -o pid", "Bash(ps *)"),
            hook(4, HookKind::Stop),
        ]);
        let SessionState::Blocked { denial } = &fleet.sessions[&SessionId(S.to_owned())].state
        else {
            panic!("not blocked");
        };
        assert_eq!(denial.candidate_rule, "Bash(ps *)");
        assert_eq!(denial.program.as_str(), "Bash:ps");
    }

    #[test]
    fn active_skill_from_slash_prompt_and_skill_tool() {
        let id = SessionId(S.to_owned());
        let from_prompt = replay(vec![start(1), prompt(2, "/fdesign plan it")]);
        let from_tool = replay(vec![
            start(1),
            prompt(2, "go"),
            hook(
                3,
                HookKind::PreToolUse(call("Skill", json!({ "skill": "fimplement" }))),
            ),
        ]);
        assert_eq!(
            from_prompt.sessions[&id].active_skill.as_deref(),
            Some("fdesign")
        );
        assert_eq!(
            from_tool.sessions[&id].active_skill.as_deref(),
            Some("fimplement")
        );
    }

    #[test]
    fn state_since_moves_only_on_change() {
        let fleet = replay(vec![
            start(1),
            prompt(2, "go"),
            hook(3, HookKind::PreToolUse(bash("ls"))),
        ]);
        assert_eq!(fleet.sessions[&SessionId(S.to_owned())].state_since, at(2));
    }

    #[test]
    fn registry_asks_for_one_exit_watch_per_new_pid() {
        let mut fleet = Fleet::new(at(0));
        assert_eq!(
            apply(&mut fleet, registry(100, "busy")),
            vec![Action::WatchExit { pid: Pid(100) }]
        );
        assert_eq!(apply(&mut fleet, registry(100, "idle")), Vec::new());
        assert_eq!(
            apply(&mut fleet, registry(101, "busy")),
            vec![Action::WatchExit { pid: Pid(101) }]
        );
    }

    #[test]
    fn sensor_since_is_stable() {
        let sensor = |second, state| Event::SensorChanged {
            sensor: SensorName::Ledger,
            state,
            detail: None,
            at: at(second),
        };
        let flapped = replay(vec![
            sensor(1, SensorState::Ok),
            sensor(2, SensorState::Ok),
            sensor(3, SensorState::Degraded),
        ]);
        let steady = replay(vec![sensor(1, SensorState::Ok), sensor(2, SensorState::Ok)]);
        assert_eq!(flapped.sensors[&SensorName::Ledger].since, at(3));
        assert_eq!(steady.sensors[&SensorName::Ledger].since, at(1));
    }

    #[test]
    fn replay_fixture() {
        let want = [
            "idle", "running", "running", "running", "running", "running", "running", "running",
            "waiting", "running", "idle", "idle", "ended",
        ];
        let lines: Vec<&str> = include_str!("../tests/fixtures/session.jsonl")
            .lines()
            .collect();
        assert_eq!(lines.len(), want.len());
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        let id = SessionId("fx-1".to_owned());
        for (index, (line, expected)) in lines.iter().zip(want).enumerate() {
            let record: Value = serde_json::from_str(line).expect("fixture line");
            let received =
                Timestamp::from_millisecond(record["at"].as_i64().expect("at")).expect("ms");
            let body = serde_json::to_vec(&record["body"]).expect("body");
            let _ = apply(
                &mut fleet,
                Event::Hook(HookEvent::parse(&body, received).expect("hook")),
            );
            let got = SessionStateWire::from(&fleet.sessions[&id].state).as_str();
            assert_eq!(got, expected, "after line {}", index + 1);
        }
        let session = &fleet.sessions[&id];
        assert_eq!(session.active_skill.as_deref(), Some("fdesign"));
        assert!(!session.subagents[&AgentId("agent-fx-a".to_owned())].running);
    }

    #[test]
    fn slash_skill_parses_only_leading_names() {
        let cases = [
            ("name", "/fdesign x", Some("fdesign")),
            (
                "plugin-name",
                "/anthropic-skills:docx",
                Some("anthropic-skills:docx"),
            ),
            ("space-after-slash", "/ x", None),
            ("mid-prompt", "run /fdesign", None),
            ("bare-slash", "/", None),
        ];
        for (label, prompt, expected) in cases {
            assert_eq!(slash_skill(prompt).as_deref(), expected, "{label}");
        }
    }

    // P3: fleets, subscriptions, digests.

    use std::path::PathBuf;

    use crate::fleet::{JournalKind, Member, SubscribeRequest};
    use crate::ids::{FleetId, MemberId, MessagingToken};
    use crate::repo::{Repo, RepoId};
    use crate::scan::{LandingGit, RepoScan};

    const O_SOCK: &str = "/tmp/o.sock";
    const WORKER: &str = "CC,FImpl - Worker - T1[WIP]";

    fn sid(name: &str) -> SessionId {
        SessionId(name.to_owned())
    }

    fn fid(name: &str) -> FleetId {
        FleetId(name.to_owned())
    }

    fn rid(path: &str) -> RepoId {
        RepoId(PathBuf::from(path))
    }

    /// A fleet with one registered `Repo` per path (P2's registration).
    /// Registered repos at `paths`, `develop` as the configured staging target.
    fn repo_fleet(paths: &[&str]) -> Fleet {
        let mut fleet = Fleet::new(at(0));
        fleet.staging = Some(crate::fleet::StagingTarget("develop".to_owned()));
        for path in paths {
            fleet.repos.insert(rid(path), Repo::new("r".to_owned()));
        }
        fleet
    }

    /// A registry re-read adds the new repo (watched, scanned once), renames the kept one and drops the gone one.
    #[test]
    fn registry_reread_follows_the_registered_set() {
        let mut fleet = repo_fleet(&["/r1", "/gone"]);
        let changed = Event::ReposChanged {
            repos: vec![
                (rid("/r1"), "one".to_owned()),
                (rid("/r2"), "two".to_owned()),
            ],
            at: at(1),
        };
        let actions = apply(&mut fleet, changed);
        let names: Vec<(&RepoId, &str)> = fleet
            .repos
            .iter()
            .map(|(id, repo)| (id, repo.name.as_str()))
            .collect();
        assert_eq!(names, [(&rid("/r1"), "one"), (&rid("/r2"), "two")]);
        assert_eq!(
            actions[0],
            Action::WatchRepos {
                repos: vec![rid("/r1"), rid("/r2")],
            }
        );
        let scanned: Vec<&RepoId> = actions
            .iter()
            .filter_map(|action| match action {
                Action::ScanRepo { repo, .. } => Some(repo),
                _ => None,
            })
            .collect();
        assert_eq!(scanned, [&rid("/r2")], "{actions:?}");
    }

    /// A registry upsert with a title, cwd and socket.
    fn titled(pid: u32, session: &str, title: &str, cwd: &str, socket: Option<&str>) -> Event {
        Event::RegistryUpserted {
            entry: RegistryEntry {
                pid: Pid(pid),
                session_id: sid(session),
                cwd: cwd.into(),
                name: Some(title.to_owned()),
                status: Some("idle".to_owned()),
                waiting_for: None,
                status_updated_at: None,
                messaging_socket_path: socket.map(PathBuf::from),
            },
            at: at(1),
        }
    }

    fn hook_as(session: &str, second: i64, kind: HookKind) -> Event {
        Event::Hook(HookEvent {
            session_id: sid(session),
            agent: None,
            cwd: None,
            transcript_path: None,
            at: at(second),
            kind,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        })
    }

    fn prompt_as(session: &str, second: i64) -> Event {
        hook_as(
            session,
            second,
            HookKind::UserPromptSubmit {
                prompt: "go".to_owned(),
            },
        )
    }

    fn start_as(session: &str, second: i64) -> Event {
        hook_as(
            session,
            second,
            HookKind::SessionStart {
                source: "startup".to_owned(),
            },
        )
    }

    fn end_as(session: &str, second: i64) -> Event {
        hook_as(
            session,
            second,
            HookKind::SessionEnd {
                reason: "other".to_owned(),
            },
        )
    }

    fn request(session: &str, fleet: &str, target: &str, socket: &str) -> SubscribeRequest {
        SubscribeRequest {
            session: sid(session),
            fleet: fid(fleet),
            target: target.to_owned(),
            socket: PathBuf::from(socket),
            token: MessagingToken("t".to_owned()),
            outside_collisions: false,
        }
    }

    fn subscribe(second: i64, session: &str, fleet: &str, target: &str, socket: &str) -> Event {
        Event::Subscribed {
            request: request(session, fleet, target, socket),
            source: SubscriptionSource::Request {
                candidate: candidate(second, fleet),
            },
            at: at(second),
        }
    }

    /// The id an edge would mint for a request at `second` naming `fleet`: distinct per second, like `now_v7`.
    fn candidate(second: i64, fleet: &str) -> InstanceId {
        InstanceId::legacy(&fid(&format!("{fleet}-request")), at(second))
    }

    fn replayed(second: i64, session: &str, fleet: &str, repo: &str, target: &str) -> Event {
        Event::Subscribed {
            request: request(session, fleet, target, O_SOCK),
            source: SubscriptionSource::Replay {
                repo: rid(repo),
                announced: BTreeSet::new(),
            },
            at: at(second),
        }
    }

    fn delivered(second: i64, session: &str, n: u64, outcome: Outcome) -> Event {
        Event::Delivered {
            session: sid(session),
            digest: DigestId(n),
            outcome,
            at: at(second),
        }
    }

    /// A `POST /worker` of `session` with token `t`: T1's worker `branch`, delivered or running.
    fn worker_as(second: i64, session: &str, branch: &str, delivered: bool) -> Event {
        worker_with(second, session, "t", branch, delivered)
    }

    fn worker_with(
        second: i64,
        session: &str,
        token: &str,
        branch: &str,
        delivered: bool,
    ) -> Event {
        Event::WorkerSet {
            fleet: fid("T1"),
            branch: WorkerBranch(branch.to_owned()),
            delivered,
            by: sid(session),
            token: Some(MessagingToken(token.to_owned())),
            at: at(second),
        }
    }

    fn worker_key(branch: &str) -> MemberKey {
        (sid("o"), Some(WorkerBranch(branch.to_owned())))
    }

    fn run(fleet: &mut Fleet, events: Vec<Event>) -> Vec<Action> {
        let mut actions = Vec::new();
        for event in events {
            actions.extend(apply(fleet, event));
        }
        actions
    }

    fn state_of(fleet: &Fleet, session: &str) -> Option<SubscriptionState> {
        fleet
            .subscriptions
            .get(&sid(session))
            .map(|subscription| subscription.state.clone())
    }

    fn fetches(actions: &[Action]) -> Vec<SessionId> {
        let fetch = |action: &Action| {
            if let Action::FetchPeek { session } = action {
                Some(session.clone())
            } else {
                None
            }
        };
        actions.iter().filter_map(fetch).collect()
    }

    fn digests(actions: &[Action]) -> Vec<Digest> {
        let deliver = |action: &Action| {
            if let Action::DeliverDigest(digest) = action {
                Some(digest.clone())
            } else {
                None
            }
        };
        actions.iter().filter_map(deliver).collect()
    }

    fn scans(actions: &[Action]) -> Vec<Vec<String>> {
        let scan = |action: &Action| {
            if let Action::ScanRepo { train_bases, .. } = action {
                Some(train_bases.clone())
            } else {
                None
            }
        };
        actions.iter().filter_map(scan).collect()
    }

    fn records(actions: &[Action]) -> Vec<Record> {
        let record = |action: &Action| {
            if let Action::Record(record) = action {
                Some(record.clone())
            } else {
                None
            }
        };
        actions.iter().filter_map(record).collect()
    }

    /// A `POST /join` of `session` to `fleet` with token `t`.
    fn join_as(second: i64, session: &str, fleet: &str) -> Event {
        Event::Joined {
            session: sid(session),
            fleet: fid(fleet),
            token: MessagingToken("t".to_owned()),
            source: JoinSource::Request {
                candidate: candidate(second, fleet),
            },
            at: at(second),
        }
    }

    /// `o` (orchestrator, no letter) idle in `/r1` and subscribed to T1 `main`; `w` joined T1 (member A), idle in
    /// `/r1`.
    fn orchestrated() -> Fleet {
        let mut fleet = repo_fleet(&["/r1", "/r2"]);
        let _ = run(
            &mut fleet,
            vec![
                titled(10, "o", "CC,FD - Orchestrator - [WIP]", "/r1", Some(O_SOCK)),
                start_as("o", 2),
                titled(11, "w", WORKER, "/r1", None),
                start_as("w", 3),
                subscribe(4, "o", "T1", "main", O_SOCK),
                join_as(5, "w", "T1"),
            ],
        );
        fleet
    }

    #[test]
    fn peek_fetch_triggers() {
        let peek = |second, state| Event::SensorChanged {
            sensor: SensorName::Peek,
            state,
            detail: None,
            at: at(second),
        };
        let agentwt = |second, state| Event::SensorChanged {
            sensor: SensorName::Agentwt,
            state,
            detail: None,
            at: at(second),
        };
        let cases: Vec<(&str, Vec<Event>, Vec<SessionId>)> = vec![
            (
                "non-subscriber-stop",
                vec![prompt_as("w", 10), hook_as("w", 11, HookKind::Stop)],
                vec![],
            ),
            (
                "subscriber-stop",
                vec![prompt_as("o", 10), hook_as("o", 11, HookKind::Stop)],
                vec![sid("o")],
            ),
            ("subscriber-prompt", vec![prompt_as("o", 10)], vec![]),
            (
                "peek-recovers",
                vec![peek(10, SensorState::Degraded), peek(11, SensorState::Ok)],
                vec![sid("o")],
            ),
            (
                "peek-stays-ok",
                vec![peek(10, SensorState::Ok), peek(11, SensorState::Ok)],
                vec![],
            ),
            (
                "agentwt-recovers",
                vec![
                    agentwt(10, SensorState::Degraded),
                    agentwt(11, SensorState::Ok),
                ],
                vec![],
            ),
        ];
        for (label, events, want) in cases {
            let mut fleet = orchestrated();
            let mut last = Vec::new();
            for event in events {
                last = apply(&mut fleet, event);
            }
            assert_eq!(fetches(&last), want, "{label}");
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn subscribe_admission() {
        let refused = |reason: &str| Some(SubscriptionState::Refused(reason.to_owned()));

        let mut fleet = orchestrated();
        let _ = apply(&mut fleet, subscribe(5, "ghost", "T1", "main", O_SOCK));
        let unknown = refused("unknown session: no hook or registry entry seen for it");
        assert_eq!(state_of(&fleet, "ghost"), unknown, "unknown");
        let _ = apply(&mut fleet, prompt_as("w", 6));
        assert_eq!(
            state_of(&fleet, "ghost"),
            unknown,
            "kept after a later event"
        );

        let mut crowd = orchestrated();
        let max_refused_unknown = crowd.caps.max_refused_unknown();
        for n in 0..=max_refused_unknown {
            let ghost = format!("ghost{n:02}");
            let second = 10 + i64::try_from(n).expect("small");
            let _ = apply(&mut crowd, subscribe(second, &ghost, "T1", "main", O_SOCK));
        }
        let ghosts = crowd
            .subscriptions
            .keys()
            .filter(|id| id.0.starts_with("ghost"))
            .count();
        assert_eq!(ghosts, max_refused_unknown, "33 refusals keep 32");
        assert!(
            !crowd.subscriptions.contains_key(&sid("ghost00")),
            "oldest dropped"
        );

        let mut ended = orchestrated();
        let _ = run(
            &mut ended,
            vec![
                start_as("e", 5),
                end_as("e", 6),
                subscribe(7, "e", "T1", "main", O_SOCK),
            ],
        );
        assert_eq!(state_of(&ended, "e"), refused("the session ended"), "ended");

        let mut mismatch = repo_fleet(&["/r1"]);
        let _ = run(
            &mut mismatch,
            vec![
                titled(10, "o", "CC - o - [WIP]", "/r1", Some(O_SOCK)),
                subscribe(5, "o", "T1", "main", "/tmp/x.sock"),
            ],
        );
        assert_eq!(
            state_of(&mismatch, "o"),
            refused("/tmp/x.sock is not the session's registered socket"),
            "socket mismatch"
        );

        let mut elsewhere = repo_fleet(&["/r1"]);
        let _ = run(
            &mut elsewhere,
            vec![
                titled(12, "z", "CC - z - [WIP]", "/elsewhere", None),
                subscribe(5, "z", "T1", "main", O_SOCK),
            ],
        );
        assert_eq!(
            state_of(&elsewhere, "z"),
            refused("session cwd /elsewhere is in no registered repo"),
            "no registered repo"
        );
        assert_eq!(elsewhere.subscriptions[&sid("z")].repo, None);

        let mut admitted = repo_fleet(&["/r1"]);
        let _ = apply(
            &mut admitted,
            titled(10, "o", "CC - o - [WIP]", "/r1", Some(O_SOCK)),
        );
        let actions = apply(&mut admitted, subscribe(5, "o", "T1", "main", O_SOCK));
        let subscription = &admitted.subscriptions[&sid("o")];
        assert_eq!(subscription.state, SubscriptionState::Active, "match");
        assert_eq!(subscription.repo, Some(rid("/r1")));
        assert_eq!(subscription.target, "main");
        assert_eq!(
            records(&actions),
            vec![
                started_record_of("T1", 5),
                Record::Subscribed {
                    session_id: sid("o"),
                    fleet: fid("T1"),
                    repo: PathBuf::from("/r1"),
                    target: "main".to_owned(),
                    socket_path: PathBuf::from(O_SOCK),
                    token: MessagingToken("t".to_owned()),
                    outside_collisions: false,
                }
            ],
            "the run starts; the operator gets no letter"
        );
        assert_eq!(fetches(&actions), vec![sid("o")]);

        let _ = run(
            &mut admitted,
            vec![
                titled(13, "p", "CC - p - [WIP]", "/r1", None),
                subscribe(6, "p", "T1", "develop", O_SOCK),
            ],
        );
        assert_eq!(
            state_of(&admitted, "p"),
            refused("fleet T1 is bound to /r1 main"),
            "other session, other target"
        );

        let _ = apply(&mut admitted, subscribe(7, "o", "T1", "develop", O_SOCK));
        assert_eq!(state_of(&admitted, "o"), Some(SubscriptionState::Active));
        assert_eq!(
            admitted.subscriptions[&sid("o")].target,
            "develop",
            "replaced"
        );

        let again = apply(&mut admitted, subscribe(8, "o", "T1", "develop", O_SOCK));
        assert_eq!(again, Vec::new(), "identical re-subscribe");

        let over = apply(
            &mut admitted,
            subscribe(9, "o", "T1", "develop", "/tmp/x.sock"),
        );
        assert!(
            matches!(
                state_of(&admitted, "o"),
                Some(SubscriptionState::Refused(_))
            ),
            "refused over active"
        );
        assert!(
            records(&over)
                .iter()
                .any(|record| matches!(record, Record::Unsubscribed { .. })),
            "{over:?}"
        );

        let replay = apply(&mut admitted, replayed(10, "ghost", "T5", "/r1", "main"));
        assert_eq!(
            state_of(&admitted, "ghost"),
            Some(SubscriptionState::Active)
        );
        assert_eq!(admitted.subscriptions[&sid("ghost")].repo, Some(rid("/r1")));
        assert!(records(&replay).is_empty(), "replay records nothing");
    }

    fn scanned(second: i64) -> Event {
        scanned_with(second, LandingGit::default())
    }

    fn scanned_with(second: i64, landing: LandingGit) -> Event {
        Event::RepoScanned {
            repo: rid("/r1"),
            started_at: at(second),
            in_flight: 1,
            outcome: Ok(Box::new(RepoScan {
                default_branch: "main".to_owned(),
                locals: vec!["main".to_owned()],
                rows: Vec::new(),
                pairs: Vec::new(),
                trains: Vec::new(),
                requested: Vec::new(),
                warnings: Vec::new(),
                work: Vec::new(),
                landing,
            })),
            at: at(second),
        }
    }

    #[test]
    fn train_bases_follow_subscriptions() {
        let mut fleet = repo_fleet(&["/r1"]);
        let _ = run(
            &mut fleet,
            vec![
                titled(10, "o", "CC - o - [WIP]", "/r1", None),
                titled(11, "p", "CC - p - [WIP]", "/r1", None),
                titled(12, "q", "CC - q - [WIP]", "/r1", None),
            ],
        );
        let bases = |fleet: &Fleet| fleet.repos[&rid("/r1")].train_bases.clone();
        // The merge branch carries the run's uuid, known only once the subscribe started the run.
        let merge = |fleet: &Fleet, name: &str| {
            fleet
                .merge_branch(&FleetId(name.to_owned()))
                .expect("bound")
        };

        let first = apply(&mut fleet, subscribe(5, "o", "T1", "develop", O_SOCK));
        let t1 = merge(&fleet, "T1");
        assert!(t1.starts_with("claude-orch/T1-"), "{t1}");
        let first_bases = vec![t1.clone(), "develop".to_owned()];
        assert_eq!(
            bases(&fleet),
            first_bases,
            "first subscription: target and merge branch"
        );
        assert_eq!(scans(&first), vec![first_bases.clone()]);
        let _ = apply(&mut fleet, scanned(6));

        let same = apply(&mut fleet, subscribe(7, "p", "T2", "develop", O_SOCK));
        let t2 = merge(&fleet, "T2");
        let two_fleets = vec![t1.clone(), t2.clone(), "develop".to_owned()];
        assert_eq!(bases(&fleet), two_fleets, "same target, own merge branch");
        assert_eq!(scans(&same), vec![two_fleets.clone()]);
        let _ = apply(&mut fleet, scanned(8));

        let third = apply(&mut fleet, subscribe(9, "q", "T3", "main", O_SOCK));
        let three_fleets = vec![
            t1,
            t2,
            merge(&fleet, "T3"),
            "develop".to_owned(),
            "main".to_owned(),
        ];
        assert_eq!(bases(&fleet), three_fleets, "third base");
        assert_eq!(scans(&third), vec![three_fleets.clone()]);
        let _ = apply(&mut fleet, scanned(10));

        let ended = apply(&mut fleet, end_as("q", 11));
        assert_eq!(bases(&fleet), two_fleets, "shrinks on end");
        assert_eq!(scans(&ended), vec![two_fleets]);
        let _ = apply(&mut fleet, scanned(12));

        let unrelated = apply(&mut fleet, prompt_as("p", 13));
        assert_eq!(unrelated, Vec::new(), "unrelated hook");

        let mut plain = repo_fleet(&["/r1"]);
        let with_develop = LandingGit {
            exists: true,
            releasable: true,
            unreleased: 0,
        };
        let seen = apply(&mut plain, scanned_with(1, with_develop));
        let develop = vec!["develop".to_owned()];
        assert_eq!(
            bases(&plain),
            develop,
            "scan with develop and no fleet: develop requested"
        );
        assert_eq!(scans(&seen), vec![develop], "and a scan follows");
        let gone = apply(&mut plain, scanned_with(2, LandingGit::default()));
        assert!(bases(&plain).is_empty(), "scan without develop: dropped");
        assert_eq!(scans(&gone), vec![Vec::<String>::new()]);
    }

    /// `h` and `w` live in `/r1` with sockets, the registry seeded, `h` holding develop's turn and `w` waiting.
    fn landing_fleet() -> Fleet {
        let mut fleet = repo_fleet(&["/r1"]);
        let request = |second: i64, session: &str| Event::LandRequested {
            session: sid(session),
            target: crate::land::LandTarget::Named("develop".to_owned()),
            token: MessagingToken(format!("tok-{session}")),
            source: crate::land::LandSource::Request,
            at: at(second),
        };
        let _ = run(
            &mut fleet,
            vec![
                titled(20, "h", "CC - h - [WIP]", "/r1/wt-h", Some("/tmp/h.sock")),
                titled(21, "w", "CC - w - [WIP]", "/r1/wt-w", Some("/tmp/w.sock")),
                seeded(2),
                request(3, "h"),
                request(4, "w"),
            ],
        );
        fleet
    }

    fn landing_of(fleet: &Fleet) -> (Option<String>, Vec<String>) {
        let key = crate::land::LandingKey {
            repo: rid("/r1"),
            target: "develop".to_owned(),
        };
        let landing = fleet.landings.turns.get(&key);
        let holder = landing
            .and_then(|landing| landing.holder.as_ref())
            .map(|turn| turn.session.0.clone());
        let queue = landing
            .map(|landing| {
                landing
                    .queue
                    .iter()
                    .map(|turn| turn.session.0.clone())
                    .collect()
            })
            .unwrap_or_default();
        (holder, queue)
    }

    #[test]
    fn land_prompt_keeps_entry() {
        let cases = [
            ("root prompt of a waiter", "w"),
            ("root prompt of the holder", "h"),
        ];
        for (label, session) in cases {
            let mut fleet = landing_fleet();
            let actions = apply(&mut fleet, prompt_as(session, 5));
            assert_eq!(
                landing_of(&fleet),
                (Some("h".to_owned()), vec!["w".to_owned()]),
                "{label}"
            );
            assert!(records(&actions).is_empty(), "{label}: no record");
        }
    }

    #[test]
    fn land_ends_with_session() {
        let mut fleet = landing_fleet();
        let actions = apply(&mut fleet, end_as("h", 5));
        assert_eq!(
            landing_of(&fleet),
            (Some("w".to_owned()), Vec::new()),
            "session end of the holder"
        );
        assert_eq!(
            records(&actions),
            vec![Record::LandEnded {
                session_id: sid("h"),
                repo: PathBuf::from("/r1"),
                target: "develop".to_owned(),
                outcome: crate::land::LandOutcome::SessionGone,
            }]
        );
        let grants: Vec<&crate::action::GrantNotice> = actions
            .iter()
            .filter_map(|action| match action {
                Action::AnnounceGrant(notice) => Some(notice),
                _ => None,
            })
            .collect();
        assert_eq!(grants.len(), 1, "one grant in the same apply");
        assert_eq!(grants[0].session, sid("w"));
        assert_eq!(grants[0].socket, Some(PathBuf::from("/tmp/w.sock")));

        let mut fleet = landing_fleet();
        let _ = apply(&mut fleet, exit(21, 5, ExitSighting::Observed));
        assert_eq!(
            landing_of(&fleet),
            (Some("h".to_owned()), Vec::new()),
            "process exit of a waiter"
        );
    }

    #[test]
    fn foreign_join_is_no_member() {
        let mut fleet = orchestrated();
        let replayed_join = Event::Joined {
            session: sid("x"),
            fleet: fid("T1"),
            token: MessagingToken("t".to_owned()),
            source: JoinSource::Replay {
                member: MemberId("C".to_owned()),
            },
            at: at(12),
        };
        let _ = run(
            &mut fleet,
            vec![titled(12, "x", WORKER, "/r2/x", None), replayed_join],
        );
        let in_t1 = |fleet: &Fleet| {
            fleet
                .members(&fid("T1"))
                .into_iter()
                .map(|Member { session, .. }| session)
                .collect::<Vec<_>>()
        };
        assert!(!in_t1(&fleet).contains(&sid("x")), "bound: not a member");
        assert_eq!(fleet.foreign(&fid("T1")), vec![sid("x")]);
        fleet.subscriptions.clear();
        assert!(in_t1(&fleet).contains(&sid("x")), "unbound: member by join");
        assert_eq!(fleet.foreign(&fid("T1")), Vec::<SessionId>::new());
    }

    #[test]
    fn digest_flow() {
        let mut idle = orchestrated();
        let actions = run(
            &mut idle,
            vec![prompt_as("w", 10), hook_as("w", 11, HookKind::Stop)],
        );
        let sent = digests(&actions);
        assert_eq!(sent.len(), 1, "idle subscriber");
        assert_eq!(sent[0].id, DigestId(1));
        assert!(
            sent[0].text.contains(&format!("- idle(T1-MA {WORKER})")),
            "{}",
            sent[0].text
        );

        let mut busy = orchestrated();
        let held = run(
            &mut busy,
            vec![
                prompt_as("o", 10),
                prompt_as("w", 11),
                hook_as("w", 12, HookKind::Stop),
            ],
        );
        assert!(digests(&held).is_empty(), "running subscriber holds");
        let released = apply(&mut busy, hook_as("o", 13, HookKind::Stop));
        assert_eq!(digests(&released).len(), 1, "released by its stop");

        let in_flight = run(
            &mut busy,
            vec![
                prompt_as("w", 14),
                hook_as("w", 15, HookKind::PermissionRequest(bash("rm x"))),
            ],
        );
        assert!(digests(&in_flight).is_empty(), "held while in flight");
        let flushed = apply(&mut busy, delivered(16, "o", 1, Outcome::Sent));
        let next = digests(&flushed);
        assert_eq!(next.len(), 1, "flushed once delivered");
        assert_eq!(next[0].id, DigestId(2));
        assert!(next[0].text.contains("waiting("), "{}", next[0].text);

        let mut stale = orchestrated();
        let _ = run(
            &mut stale,
            vec![
                prompt_as("o", 10),
                prompt_as("w", 11),
                hook_as("w", 12, HookKind::Stop),
                prompt_as("w", 13),
            ],
        );
        let dropped = apply(&mut stale, hook_as("o", 14, HookKind::Stop));
        assert!(digests(&dropped).is_empty(), "stale item dropped");

        let mut two = orchestrated();
        let _ = run(
            &mut two,
            vec![
                titled(12, "w2", "CC,FImpl - Second - T1[WIP]", "/r1", None),
                start_as("w2", 5),
                join_as(6, "w2", "T1"),
                prompt_as("o", 10),
                prompt_as("w", 11),
                hook_as("w", 12, HookKind::Stop),
                prompt_as("w2", 13),
                hook_as("w2", 14, HookKind::Stop),
            ],
        );
        let both = digests(&apply(&mut two, hook_as("o", 15, HookKind::Stop)));
        assert_eq!(both.len(), 1, "one digest");
        assert_eq!(both[0].items, 2, "two lines");
    }

    #[test]
    fn first_appearance_is_silent() {
        let mut fleet = orchestrated();
        let actions = run(
            &mut fleet,
            vec![
                titled(12, "w2", "CC,FImpl - Second - T1[WIP]", "/r1", None),
                join_as(6, "w2", "T1"),
                worker_as(10, "o", "claude/t1-a", false),
                worker_as(10, "o", "claude/t1-b", true),
            ],
        );
        assert!(digests(&actions).is_empty(), "{actions:?}");
        assert!(fleet.subscriptions[&sid("o")].pending.is_empty());
    }

    #[test]
    fn subscription_ends() {
        let ended =
            |fleet: &Fleet| matches!(state_of(fleet, "o"), Some(SubscriptionState::Ended(_)));

        let mut session_end = orchestrated();
        let actions = apply(&mut session_end, end_as("o", 10));
        assert!(ended(&session_end), "session end");
        assert_eq!(
            records(&actions),
            vec![Record::Unsubscribed {
                session_id: sid("o"),
                reason: "the session ended".to_owned(),
            }]
        );

        let mut moved = orchestrated();
        let _ = apply(
            &mut moved,
            titled(
                10,
                "o",
                "CC,FD - Orchestrator - [WIP]",
                "/r1",
                Some("/tmp/o2.sock"),
            ),
        );
        assert!(ended(&moved), "socket changed");

        let mut refused = orchestrated();
        let _ = run(
            &mut refused,
            vec![prompt_as("w", 10), hook_as("w", 11, HookKind::Stop)],
        );
        let _ = apply(
            &mut refused,
            delivered(12, "o", 1, Outcome::Refused("gone".to_owned())),
        );
        assert_eq!(
            state_of(&refused, "o"),
            Some(SubscriptionState::Ended("socket refused: gone".to_owned()))
        );

        let mut unconfirmed = orchestrated();
        let _ = run(
            &mut unconfirmed,
            vec![prompt_as("w", 10), hook_as("w", 11, HookKind::Stop)],
        );
        let outcome = Outcome::Unconfirmed("broken pipe".to_owned());
        let _ = apply(&mut unconfirmed, delivered(12, "o", 1, outcome.clone()));
        let subscription = &unconfirmed.subscriptions[&sid("o")];
        assert_eq!(subscription.state, SubscriptionState::Active, "unconfirmed");
        assert_eq!(
            subscription.last.as_ref().map(|last| last.outcome.clone()),
            Some(outcome)
        );
    }

    fn unsubscribe(second: i64, session: &str, token: &str) -> Event {
        Event::UnsubscribeRequested {
            session: sid(session),
            token: MessagingToken(token.to_owned()),
            finish: false,
            at: at(second),
        }
    }

    #[test]
    fn unsubscribe_cases() {
        let by_session = SubscriptionState::Ended("unsubscribed by the session".to_owned());

        let mut matching = orchestrated();
        let actions = apply(&mut matching, unsubscribe(10, "o", "t"));
        assert_eq!(state_of(&matching, "o"), Some(by_session.clone()), "match");
        assert_eq!(
            records(&actions),
            vec![Record::Unsubscribed {
                session_id: sid("o"),
                reason: "unsubscribed by the session".to_owned(),
            }]
        );
        let again = apply(&mut matching, unsubscribe(11, "o", "t"));
        assert!(records(&again).is_empty(), "already ended: {again:?}");
        assert_eq!(state_of(&matching, "o"), Some(by_session), "stays ended");

        let mut wrong = orchestrated();
        let actions = apply(&mut wrong, unsubscribe(10, "o", "x"));
        assert!(actions.is_empty(), "wrong token: {actions:?}");
        assert_eq!(state_of(&wrong, "o"), Some(SubscriptionState::Active));

        let mut unknown = orchestrated();
        let actions = apply(&mut unknown, unsubscribe(10, "ghost", "t"));
        assert!(actions.is_empty(), "unknown session: {actions:?}");
        assert_eq!(state_of(&unknown, "ghost"), None);
    }

    #[test]
    fn worker_branches_recorded() {
        use crate::branch::{Branch, BranchState};
        use crate::repo::BranchKey;
        use crate::scan::ScannedRow;

        const WT_O: &str = "/r1/.claude/worktrees/o";
        const WT_W: &str = "/r1/.claude/worktrees/w";
        const WT_X: &str = "/r1/.claude/worktrees/x";
        let lane = |name: &str| BranchKey::Branch(name.to_owned());
        let row = |name: &str, worktree: &str| ScannedRow {
            key: lane(name),
            worktree: Some(PathBuf::from(worktree)),
            main: false,
            tracked_changes: 0,
            untracked: 0,
            own_commits: 1,
            fresh: false,
            from: Some("main".to_owned()),
            contained_in: None,
            safe_to_remove: false,
        };

        let mut fleet = orchestrated();
        let _ = run(
            &mut fleet,
            vec![
                titled(12, "x", "CC,FImpl - Third - T1[WIP]", "/r1", None),
                join_as(6, "x", "T1"),
                worker_as(5, "o", "claude/t1-a", false),
                worker_as(5, "o", "claude/t1-b", false),
            ],
        );
        for (session, cwd) in [("o", WT_O), ("w", WT_W), ("x", WT_X)] {
            fleet.sessions.get_mut(&sid(session)).expect("session").cwd = Some(PathBuf::from(cwd));
        }
        let repo = fleet.repos.get_mut(&rid("/r1")).expect("r1");
        repo.snapshot = Some(RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows: vec![
                row("claude/o", WT_O),
                row("claude/w", WT_W),
                row("main", WT_X),
            ],
            pairs: Vec::new(),
            trains: Vec::new(),
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: LandingGit::default(),
        });
        for name in ["claude/o", "claude/w", "claude/t1-a", "main"] {
            let branch = Branch {
                state: BranchState::Unmerged,
                since: at(0),
                judged: "main".to_owned(),
                target: None,
            };
            repo.branches.insert(lane(name), branch);
        }
        let _ = apply(&mut fleet, prompt_as("w", 20));

        let seen = &fleet.subscriptions[&sid("o")].worker_branches_seen;
        assert_eq!(
            *seen,
            BTreeSet::from([lane("claude/t1-a"), lane("claude/w")]),
            "worker rows only: no subscriber branch, no target, no row-less worker branch"
        );
    }

    #[test]
    fn digest_ids_continue_after_replay() {
        let mut fleet = repo_fleet(&["/r1"]);
        let _ = run(
            &mut fleet,
            vec![
                replayed(1, "o", "T1", "/r1", "main"),
                delivered(1, "o", 7, Outcome::Sent),
                titled(10, "o", "CC - o - [WIP]", "/r1", Some(O_SOCK)),
                start_as("o", 2),
                titled(11, "w", WORKER, "/r1", None),
                start_as("w", 3),
                join_as(3, "w", "T1"),
                prompt_as("w", 4),
            ],
        );
        let sent = digests(&apply(&mut fleet, hook_as("w", 5, HookKind::Stop)));
        assert_eq!(
            sent.iter().map(|digest| digest.id).collect::<Vec<_>>(),
            vec![DigestId(8)]
        );
    }

    fn members_of(fleet: &Fleet, id: &str) -> Vec<SessionId> {
        fleet
            .members(&fid(id))
            .into_iter()
            .map(|member| member.session)
            .collect()
    }

    fn letter(fleet: &Fleet, id: &str, session: &str, worker: Option<&str>) -> Option<String> {
        let key = (
            sid(session),
            worker.map(|branch| WorkerBranch(branch.to_owned())),
        );
        fleet
            .member_id(&fid(id), &key)
            .map(|member| member.0.clone())
    }

    fn joined_record(session: &str, fleet: &str, member: &str) -> Record {
        Record::Joined {
            session_id: sid(session),
            fleet: fid(fleet),
            member: MemberId(member.to_owned()),
            token: MessagingToken("t".to_owned()),
        }
    }

    /// The `fleet_started` record a request at `second` writes when it starts `fleet`'s run.
    fn started_record_of(fleet: &str, second: i64) -> Record {
        Record::FleetStarted {
            fleet: fid(fleet),
            instance: candidate(second, fleet),
        }
    }

    #[test]
    fn join_cases() {
        let mut fleet = repo_fleet(&["/r1", "/r2"]);
        let _ = run(
            &mut fleet,
            vec![
                titled(10, "a", "CC - a - [WIP]", "/r1", None),
                titled(11, "b", "CC - b - [WIP]", "/r1", None),
                titled(12, "o", "CC - o - [WIP]", "/r1", Some(O_SOCK)),
                titled(13, "z", "CC - z - [WIP]", "/r2", None),
                start_as("e", 1),
                end_as("e", 2),
            ],
        );
        let refusal = |fleet: &Fleet, session: &str| {
            fleet
                .join_refusals
                .get(&sid(session))
                .map(|(_, reason)| reason.clone())
        };

        let first = apply(&mut fleet, join_as(3, "a", "T1"));
        assert_eq!(
            records(&first),
            vec![started_record_of("T1", 3), joined_record("a", "T1", "A")],
            "first starts the run"
        );
        assert_eq!(members_of(&fleet, "T1"), vec![sid("a")]);

        let second = apply(&mut fleet, join_as(4, "b", "T1"));
        assert_eq!(
            records(&second),
            vec![joined_record("b", "T1", "B")],
            "second"
        );

        let again = apply(&mut fleet, join_as(5, "a", "T1"));
        assert_eq!(
            records(&again),
            vec![joined_record("a", "T1", "A")],
            "re-join keeps A"
        );

        let other = apply(&mut fleet, join_as(6, "a", "T2"));
        assert_eq!(
            records(&other),
            vec![started_record_of("T2", 6), joined_record("a", "T2", "A")],
            "other fleet"
        );
        assert_eq!(members_of(&fleet, "T1"), vec![sid("b")], "left T1");
        assert_eq!(members_of(&fleet, "T2"), vec![sid("a")], "in T2");
        assert_eq!(
            letter(&fleet, "T1", "a", None),
            Some("A".to_owned()),
            "never reused within the run"
        );

        let ghost = apply(&mut fleet, join_as(7, "ghost", "T1"));
        assert!(records(&ghost).is_empty(), "unknown: {ghost:?}");
        assert_eq!(
            refusal(&fleet, "ghost").as_deref(),
            Some("unknown session: no hook or registry entry seen for it")
        );

        let ended = apply(&mut fleet, join_as(8, "e", "T1"));
        assert!(records(&ended).is_empty(), "ended: {ended:?}");
        assert_eq!(refusal(&fleet, "e").as_deref(), Some("the session ended"));

        let _ = apply(&mut fleet, subscribe(9, "o", "T1", "main", O_SOCK));
        let elsewhere = apply(&mut fleet, join_as(10, "z", "T1"));
        assert!(
            records(&elsewhere).is_empty(),
            "bound elsewhere: {elsewhere:?}"
        );
        assert_eq!(
            refusal(&fleet, "z").as_deref(),
            Some("fleet T1 is bound to /r1")
        );
        assert!(!fleet.joins.contains_key(&sid("z")));
    }

    #[test]
    fn member_ids_assigned() {
        let mut fleet = orchestrated();
        assert_eq!(letter(&fleet, "T1", "o", None), None, "the operator");
        assert_eq!(
            letter(&fleet, "T1", "w", None),
            Some("A".to_owned()),
            "joined"
        );

        let actions = run(
            &mut fleet,
            vec![
                worker_as(10, "o", "claude/t1-b", false),
                worker_as(10, "o", "claude/t1-a", false),
            ],
        );
        let assigned = |branch: &str, member: &str| Record::MemberAssigned {
            fleet: fid("T1"),
            session_id: sid("o"),
            worker: Some(WorkerBranch(branch.to_owned())),
            agent_id: None,
            member: MemberId(member.to_owned()),
        };
        let worker_line = |branch: &str| Record::Worker {
            fleet: fid("T1"),
            session_id: sid("o"),
            branch: WorkerBranch(branch.to_owned()),
            delivered: false,
        };
        let lettered: Vec<Record> = records(&actions)
            .into_iter()
            .filter(|record| {
                matches!(
                    record,
                    Record::MemberAssigned { .. } | Record::Worker { .. }
                )
            })
            .collect();
        assert_eq!(
            lettered,
            vec![
                worker_line("claude/t1-b"),
                assigned("claude/t1-b", "B"),
                worker_line("claude/t1-a"),
                assigned("claude/t1-a", "C"),
            ],
            "worker-letter-at-registration: registration order, the worker line before its letter line"
        );

        let _ = apply(&mut fleet, end_as("w", 11));
        for n in 0..fleet.caps.max_terminal() {
            let other = format!("t{n:04}");
            let _ = apply(&mut fleet, start_as(&other, 20));
            let _ = apply(&mut fleet, end_as(&other, 21));
        }
        assert!(!fleet.joins.contains_key(&sid("w")), "evicted join");
        let _ = run(
            &mut fleet,
            vec![
                titled(30, "n", "CC - n - [WIP]", "/r1", None),
                join_as(30, "n", "T1"),
            ],
        );
        assert_eq!(
            letter(&fleet, "T1", "n", None),
            Some("D".to_owned()),
            "A not reused"
        );
    }

    fn done_as(second: i64, session: &str, token: &str) -> Event {
        Event::DoneReported {
            session: sid(session),
            token: Some(MessagingToken(token.to_owned())),
            at: at(second),
        }
    }

    #[test]
    fn done_cases() {
        let delivered_record = vec![Record::Delivered {
            session_id: sid("w"),
        }];
        let is_delivered = |fleet: &Fleet| fleet.delivered.contains_key(&sid("w"));

        let mut fleet = orchestrated();
        let done = apply(&mut fleet, done_as(10, "w", "t"));
        assert_eq!(records(&done), delivered_record, "matching token");
        assert!(is_delivered(&fleet));

        let twice = apply(&mut fleet, done_as(11, "w", "t"));
        assert!(records(&twice).is_empty(), "second done: {twice:?}");

        fleet
            .sessions
            .get_mut(&sid("w"))
            .expect("w")
            .subagents
            .insert(
                AgentId("a1".to_owned()),
                Subagent {
                    agent_type: "Explore".to_owned(),
                    running: true,
                    started_at: at(12),
                    model: None,
                    requested: Requested::Unknown,
                },
            );
        let Event::Hook(mut sub_prompt) = prompt_as("w", 12) else {
            unreachable!("prompt_as builds Event::Hook")
        };
        sub_prompt.agent = Some(Agent {
            id: AgentId("a1".to_owned()),
            agent_type: "Explore".to_owned(),
        });
        let _ = apply(&mut fleet, Event::Hook(sub_prompt));
        assert!(is_delivered(&fleet), "subagent prompt keeps it delivered");

        let resumed = apply(&mut fleet, prompt_as("w", 13));
        assert_eq!(
            records(&resumed),
            vec![Record::Resumed {
                session_id: sid("w")
            }],
            "root prompt"
        );
        assert!(!is_delivered(&fleet));

        let mut wrong = orchestrated();
        let actions = apply(&mut wrong, done_as(10, "w", "x"));
        assert!(records(&actions).is_empty(), "wrong token: {actions:?}");
        assert!(!is_delivered(&wrong));

        let mut not_joined = orchestrated();
        let actions = apply(&mut not_joined, done_as(10, "o", "t"));
        assert!(records(&actions).is_empty(), "not joined: {actions:?}");
        assert!(not_joined.delivered.is_empty());
    }

    fn journal_as(second: i64, session: &str, token: &str, subject: &str) -> Event {
        Event::JournalRecorded {
            fleet: fid("T1"),
            entry: JournalEntry {
                kind: JournalKind::Merged,
                members: vec![MemberId("B".to_owned())],
                subject: subject.to_owned(),
                detail: None,
                by: sid(session),
                at: at(second),
            },
            token: Some(MessagingToken(token.to_owned())),
        }
    }

    #[test]
    fn journal_cases() {
        let journal_len = |fleet: &Fleet| {
            fleet
                .instances
                .get(&fid("T1"))
                .map_or(0, |instance| instance.journal.len())
        };

        let mut fleet = orchestrated();
        let stored = apply(&mut fleet, journal_as(10, "o", "t", "merged claude/w"));
        assert!(
            matches!(records(&stored).as_slice(), [Record::Journal { .. }]),
            "subscriber: {stored:?}"
        );
        assert_eq!(journal_len(&fleet), 1);

        let member = apply(&mut fleet, journal_as(11, "w", "t", "not mine"));
        assert!(records(&member).is_empty(), "member: {member:?}");
        let wrong = apply(&mut fleet, journal_as(12, "o", "x", "wrong token"));
        assert!(records(&wrong).is_empty(), "wrong token: {wrong:?}");
        assert_eq!(journal_len(&fleet), 1);

        let mut full = orchestrated();
        for n in 0..=200 {
            let second = 10 + i64::from(n);
            let _ = apply(&mut full, journal_as(second, "o", "t", &format!("n{n}")));
        }
        let journal = &full.instances[&fid("T1")].journal;
        assert_eq!(journal.len(), 201, "201 entries, all kept");
        assert_eq!(
            journal.first().map(|entry| entry.subject.as_str()),
            Some("n0"),
            "oldest kept"
        );
    }

    // Fleet runs: leave, dissolve, archive.

    fn seeded(second: i64) -> Event {
        Event::RegistrySeeded { at: at(second) }
    }

    fn leave_as(second: i64, session: &str, token: &str) -> Event {
        Event::LeaveRequested {
            session: sid(session),
            token: MessagingToken(token.to_owned()),
            at: at(second),
        }
    }

    fn unsubscribe_as(second: i64, session: &str) -> Event {
        Event::UnsubscribeRequested {
            session: sid(session),
            token: MessagingToken("t".to_owned()),
            finish: false,
            at: at(second),
        }
    }

    fn archives(actions: &[Action]) -> Vec<FleetArchive> {
        actions
            .iter()
            .filter_map(|action| match action {
                Action::Dissolve { archive, .. } => Some((**archive).clone()),
                _ => None,
            })
            .collect()
    }

    fn left_record(session: &str, reason: &str) -> Record {
        Record::Left {
            session_id: sid(session),
            reason: reason.to_owned(),
        }
    }

    /// `a` and `b` known and idle in `/r1`, the registry seeded.
    fn two_workers() -> Fleet {
        let mut fleet = repo_fleet(&["/r1"]);
        let _ = run(
            &mut fleet,
            vec![
                titled(10, "a", "CC - a - [WIP]", "/r1", None),
                titled(11, "b", "CC - b - [WIP]", "/r1", None),
                seeded(2),
            ],
        );
        fleet
    }

    #[test]
    fn instance_lifecycle() {
        let mut fleet = two_workers();
        let first = apply(&mut fleet, join_as(3, "a", "T1"));
        assert_eq!(
            records(&first),
            vec![started_record_of("T1", 3), joined_record("a", "T1", "A")]
        );
        let run_id = fleet.instances[&fid("T1")].id;
        assert_eq!(run_id, candidate(3, "T1"), "the candidate becomes the run");
        let second = apply(&mut fleet, join_as(4, "b", "T1"));
        assert_eq!(records(&second), vec![joined_record("b", "T1", "B")]);
        assert_eq!(fleet.instances[&fid("T1")].id, run_id, "same run");

        let a_left = apply(&mut fleet, leave_as(5, "a", "t"));
        assert_eq!(records(&a_left), vec![left_record("a", "left")]);
        assert!(archives(&a_left).is_empty(), "b is still a member");

        let b_left = apply(&mut fleet, leave_as(6, "b", "t"));
        let dissolved = archives(&b_left);
        assert_eq!(dissolved.len(), 1, "{b_left:?}");
        let archive = &dissolved[0];
        assert_eq!(archive.instance, run_id.to_string());
        let roster: Vec<(&str, &str, &str)> = archive
            .members
            .iter()
            .map(|member| {
                (
                    member.token.as_str(),
                    member.session_id.as_str(),
                    member.left_at.as_str(),
                )
            })
            .collect();
        let (five, six) = (at(5).to_string(), at(6).to_string());
        assert_eq!(
            roster,
            [("T1-MA", "a", five.as_str()), ("T1-MB", "b", six.as_str())]
        );
        assert!(
            !fleet.instances.contains_key(&fid("T1")),
            "dropped from memory"
        );
        assert!(
            b_left.iter().any(|action| matches!(
                action,
                Action::Dissolve { record: Record::Dissolved { instance, .. }, .. } if *instance == run_id
            )),
            "dissolved record rides with the archive"
        );

        let again = apply(&mut fleet, join_as(7, "a", "T1"));
        assert_eq!(
            records(&again),
            vec![started_record_of("T1", 7), joined_record("a", "T1", "A")],
            "a new run starts at A"
        );
        assert_ne!(fleet.instances[&fid("T1")].id, run_id);

        let mut unseeded = repo_fleet(&["/r1"]);
        let _ = run(
            &mut unseeded,
            vec![
                titled(10, "a", "CC - a - [WIP]", "/r1", None),
                join_as(3, "a", "T1"),
            ],
        );
        let actions = apply(&mut unseeded, leave_as(4, "a", "t"));
        assert!(archives(&actions).is_empty(), "not seeded: no dissolve");
        assert!(unseeded.instances.contains_key(&fid("T1")));
    }

    #[test]
    fn leave_cases() {
        let mut fleet = two_workers();
        let _ = run(
            &mut fleet,
            vec![join_as(3, "a", "T1"), join_as(4, "b", "T1")],
        );
        let wrong = apply(&mut fleet, leave_as(5, "a", "x"));
        assert!(records(&wrong).is_empty(), "wrong token: {wrong:?}");
        assert!(fleet.joins.contains_key(&sid("a")));
        let stranger = apply(&mut fleet, leave_as(5, "z", "t"));
        assert!(records(&stranger).is_empty(), "not joined: {stranger:?}");

        let ended = apply(&mut fleet, end_as("a", 6));
        assert_eq!(records(&ended), vec![left_record("a", "the session ended")]);

        let mut restarted = repo_fleet(&["/r1"]);
        let replayed_join = Event::Joined {
            session: sid("gone"),
            fleet: fid("T1"),
            token: MessagingToken("t".to_owned()),
            source: JoinSource::Replay {
                member: MemberId("A".to_owned()),
            },
            at: at(1),
        };
        let before_seed = apply(&mut restarted, replayed_join);
        assert!(records(&before_seed).is_empty(), "replay writes nothing");
        let seed = apply(&mut restarted, seeded(2));
        assert_eq!(
            records(&seed),
            vec![left_record("gone", "session gone while heartbeat was down")]
        );
        assert_eq!(archives(&seed).len(), 1, "the legacy run dissolves");
    }

    #[test]
    fn orchestrator_outlives_members() {
        let mut fleet = orchestrated();
        let _ = apply(&mut fleet, seeded(6));
        let left = apply(&mut fleet, leave_as(7, "w", "t"));
        assert!(archives(&left).is_empty(), "the subscription keeps it live");
        let ended = apply(&mut fleet, unsubscribe_as(8, "o"));
        let dissolved = archives(&ended);
        assert_eq!(dissolved.len(), 1, "{ended:?}");
        let tokens: Vec<&str> = dissolved[0]
            .members
            .iter()
            .map(|member| member.token.as_str())
            .collect();
        assert_eq!(tokens, ["T1-O", "T1-MA"]);
        assert_eq!(
            dissolved[0].subscriptions.len(),
            1,
            "the ended subscription"
        );
        assert_eq!(dissolved[0].target.as_deref(), Some("main"));

        let mut busy = orchestrated();
        let _ = apply(&mut busy, seeded(6));
        let ended = apply(&mut busy, unsubscribe_as(7, "o"));
        assert!(archives(&ended).is_empty(), "w is still joined");
        let left = apply(&mut busy, leave_as(8, "w", "t"));
        assert_eq!(archives(&left).len(), 1, "last member out");
    }

    fn remove_as(second: i64, fleet: &str, member: &str) -> Event {
        Event::RemoveRequested {
            fleet: fid(fleet),
            member: MemberId(member.to_owned()),
            at: at(second),
        }
    }

    fn member_keys(fleet: &Fleet, name: &str) -> Vec<MemberKey> {
        fleet
            .members(&fid(name))
            .into_iter()
            .map(|member| (member.session, member.worker))
            .collect()
    }

    #[test]
    fn remove_cases() {
        let mut fleet = two_workers();
        let _ = run(
            &mut fleet,
            vec![join_as(3, "a", "T1"), join_as(4, "b", "T1")],
        );
        let joined = apply(&mut fleet, remove_as(5, "T1", "A"));
        assert_eq!(
            records(&joined),
            vec![Record::Removed {
                fleet: fid("T1"),
                member: MemberId("A".to_owned()),
            }],
            "joined member: one Removed record, no Left"
        );
        assert!(!fleet.joins.contains_key(&sid("a")), "joined member left");
        assert_eq!(member_keys(&fleet, "T1"), vec![(sid("b"), None)]);
        let roster = &fleet.instances[&fid("T1")].roster;
        assert_eq!(roster[&(sid("a"), None)].left_at, Some(at(5)));

        let unknown = apply(&mut fleet, remove_as(6, "T1", "Z"));
        assert!(records(&unknown).is_empty(), "unknown letter: {unknown:?}");

        let again = apply(&mut fleet, join_as(7, "a", "T1"));
        assert_eq!(
            records(&again),
            vec![joined_record("a", "T1", "A")],
            "a new join keeps the letter"
        );
        assert!(
            member_keys(&fleet, "T1").contains(&(sid("a"), None)),
            "member again"
        );

        let mut replayed = two_workers();
        let _ = run(
            &mut replayed,
            vec![
                join_as(3, "a", "T1"),
                join_as(4, "b", "T1"),
                Event::Removed {
                    fleet: fid("T1"),
                    member: MemberId("A".to_owned()),
                    at: at(5),
                },
            ],
        );
        assert_eq!(
            member_keys(&replayed, "T1"),
            vec![(sid("b"), None)],
            "replay"
        );
        assert!(!replayed.joins.contains_key(&sid("a")));
    }

    #[test]
    fn worker_removed_by_operator() {
        let mut fleet = orchestrated();
        let subagent_start = Event::Hook(HookEvent {
            session_id: sid("o"),
            agent: Some(Agent {
                id: AgentId("a1".to_owned()),
                agent_type: "implement-runner".to_owned(),
            }),
            cwd: None,
            transcript_path: None,
            at: at(7),
            kind: HookKind::SubagentStart,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        });
        let _ = run(
            &mut fleet,
            vec![
                prompt_as("o", 6),
                subagent_start,
                worker_as(7, "o", "claude/t1-a", false),
            ],
        );
        let worker = worker_key("claude/t1-a");
        assert_eq!(
            member_keys(&fleet, "T1"),
            vec![(sid("o"), None), worker.clone(), (sid("w"), None)],
            "the worker is a member, the subagent is none"
        );
        let letter = fleet
            .member_id(&fid("T1"), &worker)
            .expect("lettered")
            .clone();
        let removed = apply(&mut fleet, remove_as(8, "T1", &letter.0));
        assert_eq!(records(&removed).len(), 1, "{removed:?}");
        assert!(!member_keys(&fleet, "T1").contains(&worker), "worker gone");
        let roster = &fleet.instances[&fid("T1")].roster;
        assert_eq!(roster[&worker].left_at, Some(at(8)), "roster keeps it");
        assert!(
            fleet.sessions[&sid("o")]
                .subagents
                .contains_key(&AgentId("a1".to_owned())),
            "the subagent itself is untouched"
        );

        let again = apply(&mut fleet, worker_as(9, "o", "claude/t1-a", false));
        assert!(
            records(&again).contains(&Record::MemberAssigned {
                fleet: fid("T1"),
                session_id: sid("o"),
                worker: Some(WorkerBranch("claude/t1-a".to_owned())),
                agent_id: None,
                member: letter.clone(),
            }),
            "re-registered after removal: its old letter line again: {again:?}"
        );
        assert!(
            member_keys(&fleet, "T1").contains(&worker),
            "a member again"
        );
        assert_eq!(
            fleet.member_id(&fid("T1"), &worker),
            Some(&letter),
            "keeps its letter"
        );
    }

    /// The ledger lines one registration writes, replayed into a fresh daemon: the same letter, and no line written.
    #[test]
    fn worker_replay_writes_no_line() {
        let mut live = orchestrated();
        let _ = apply(&mut live, seeded(6));
        let written = records(&run(
            &mut live,
            vec![
                worker_as(7, "o", "claude/t1-a", false),
                remove_as(8, "T1", "B"),
                worker_as(9, "o", "claude/t1-a", true),
            ],
        ));
        let replayed: Vec<Event> = written
            .iter()
            .filter_map(|record| match record.clone() {
                Record::Worker {
                    fleet,
                    session_id,
                    branch,
                    delivered,
                } => Some(Event::WorkerSet {
                    fleet,
                    branch,
                    delivered,
                    by: session_id,
                    token: None,
                    at: at(7),
                }),
                Record::MemberAssigned {
                    fleet,
                    session_id,
                    worker: Some(worker),
                    member,
                    ..
                } => Some(Event::MemberAssigned {
                    fleet,
                    key: (session_id, Some(worker)),
                    member,
                    at: at(7),
                }),
                Record::Removed { fleet, member } => Some(Event::Removed {
                    fleet,
                    member,
                    at: at(8),
                }),
                _ => None,
            })
            .collect();
        assert_eq!(
            replayed.len(),
            5,
            "worker, letter, removed, worker, letter: {written:?}"
        );

        let mut restarted = orchestrated();
        let lines = run(&mut restarted, replayed);
        assert!(
            !lines.iter().any(|action| matches!(
                action,
                Action::Record(Record::Worker { .. } | Record::MemberAssigned { .. })
            )),
            "replay writes no worker or letter line: {lines:?}"
        );
        let key = worker_key("claude/t1-a");
        assert_eq!(
            restarted.member_id(&fid("T1"), &key),
            live.member_id(&fid("T1"), &key),
            "the same letter"
        );
        assert!(
            member_keys(&restarted, "T1").contains(&key),
            "re-registered after removal: a member after replay too"
        );
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn worker_cases() {
        type Check = Box<dyn Fn(&Fleet, &[Action])>;
        let registered = |delivered: bool| Record::Worker {
            fleet: fid("T1"),
            session_id: sid("o"),
            branch: WorkerBranch("claude/t1-a".to_owned()),
            delivered,
        };
        let worker_of = |fleet: &Fleet| {
            fleet
                .instances
                .get(&fid("T1"))
                .and_then(|instance| {
                    instance
                        .workers
                        .get(&WorkerBranch("claude/t1-a".to_owned()))
                })
                .cloned()
        };
        let is_member =
            |fleet: &Fleet| member_keys(fleet, "T1").contains(&worker_key("claude/t1-a"));
        let cases: Vec<(&str, Vec<Event>, Check)> = vec![
            (
                "worker-registered-by-subscriber",
                vec![worker_as(10, "o", "claude/t1-a", false)],
                Box::new(move |fleet, actions| {
                    let got = records(actions);
                    assert_eq!(got.first(), Some(&registered(false)), "{got:?}");
                    assert!(is_member(fleet));
                    let worker = worker_of(fleet).expect("worker");
                    assert_eq!(
                        (worker.by, worker.since, worker.delivered),
                        (sid("o"), at(10), false)
                    );
                }),
            ),
            (
                "worker-wrong-token-refused",
                vec![worker_with(10, "o", "x", "claude/t1-a", false)],
                Box::new(move |fleet, actions| {
                    assert!(records(actions).is_empty(), "{actions:?}");
                    assert!(worker_of(fleet).is_none());
                }),
            ),
            (
                "worker-from-non-subscriber-refused",
                vec![worker_as(10, "w", "claude/t1-a", false)],
                Box::new(move |fleet, actions| {
                    assert!(records(actions).is_empty(), "{actions:?}");
                    assert!(worker_of(fleet).is_none());
                }),
            ),
            (
                "worker-from-ended-subscription-refused",
                vec![
                    unsubscribe(9, "o", "t"),
                    worker_as(10, "o", "claude/t1-a", false),
                ],
                Box::new(move |fleet, actions| {
                    assert!(
                        !records(actions)
                            .iter()
                            .any(|record| matches!(record, Record::Worker { .. })),
                        "{actions:?}"
                    );
                    assert!(worker_of(fleet).is_none());
                }),
            ),
            (
                "worker-set-delivered",
                vec![
                    worker_as(10, "o", "claude/t1-a", false),
                    worker_as(11, "o", "claude/t1-a", true),
                ],
                Box::new(move |fleet, actions| {
                    assert!(records(actions).contains(&registered(true)), "{actions:?}");
                    let worker = worker_of(fleet).expect("worker");
                    assert!(worker.delivered);
                    assert_eq!(worker.since, at(10), "registration time kept");
                    let member = Member {
                        session: sid("o"),
                        worker: Some(WorkerBranch("claude/t1-a".to_owned())),
                        via: Via::Worker,
                    };
                    assert!(fleet.is_delivered(&member));
                }),
            ),
            (
                "worker-reopened-running",
                vec![
                    worker_as(10, "o", "claude/t1-a", true),
                    worker_as(11, "o", "claude/t1-a", false),
                ],
                Box::new(move |fleet, _| {
                    assert!(!worker_of(fleet).expect("worker").delivered);
                }),
            ),
            (
                "worker-registered-twice-keeps-letter",
                vec![
                    worker_as(10, "o", "claude/t1-a", false),
                    worker_as(11, "o", "claude/t1-a", false),
                ],
                Box::new(move |fleet, actions| {
                    let letters = records(actions)
                        .iter()
                        .filter(|record| matches!(record, Record::MemberAssigned { .. }))
                        .count();
                    assert_eq!(letters, 1, "{actions:?}");
                    assert_eq!(
                        letter(fleet, "T1", "o", Some("claude/t1-a")),
                        Some("B".to_owned())
                    );
                }),
            ),
            (
                "worker-replay-trusted",
                vec![Event::WorkerSet {
                    fleet: fid("T1"),
                    branch: WorkerBranch("claude/t1-a".to_owned()),
                    delivered: true,
                    by: sid("o"),
                    token: None,
                    at: at(10),
                }],
                Box::new(move |fleet, actions| {
                    assert!(
                        !records(actions)
                            .iter()
                            .any(|record| matches!(record, Record::Worker { .. })),
                        "replay writes no worker record: {actions:?}"
                    );
                    assert!(worker_of(fleet).expect("worker").delivered);
                    assert!(is_member(fleet));
                }),
            ),
        ];
        for (label, events, check) in cases {
            let mut fleet = orchestrated();
            let actions = run(&mut fleet, events);
            let outcome = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                check(&fleet, &actions);
            }));
            assert!(outcome.is_ok(), "{label}");
        }
    }

    #[test]
    fn worker_roster_left_at_dissolve() {
        let mut fleet = orchestrated();
        let _ = run(
            &mut fleet,
            vec![
                seeded(6),
                worker_as(7, "o", "claude/t1-a", true),
                leave_as(8, "w", "t"),
            ],
        );
        let ended = apply(&mut fleet, unsubscribe_as(9, "o"));
        let dissolved = archives(&ended);
        assert_eq!(dissolved.len(), 1, "{ended:?}");
        let worker = dissolved[0]
            .members
            .iter()
            .find(|member| member.via == crate::wire::ViaWire::Worker)
            .expect("worker archived");
        assert_eq!(worker.token, "T1-MB");
        assert_eq!(worker.session_id, "o");
        assert_eq!(worker.joined_at, at(7).to_string());
        assert_eq!(worker.left_at, at(9).to_string());
        assert!(worker.delivered);
    }

    #[test]
    fn subscribe_drops_own_join() {
        let mut fleet = two_workers();
        let _ = run(
            &mut fleet,
            vec![join_as(3, "a", "T1"), join_as(4, "b", "T2")],
        );
        let subscribed = apply(&mut fleet, subscribe(5, "a", "T1", "main", O_SOCK));
        assert!(
            records(&subscribed).contains(&left_record("a", "subscribed as the fleet's operator")),
            "{subscribed:?}"
        );
        assert!(!fleet.joins.contains_key(&sid("a")));
        let other = apply(&mut fleet, subscribe(6, "b", "T1", "main", O_SOCK));
        assert!(
            !records(&other)
                .iter()
                .any(|record| matches!(record, Record::Left { .. })),
            "joined another fleet: kept"
        );
        assert!(fleet.joins.contains_key(&sid("b")));

        let mut refused = two_workers();
        let _ = run(
            &mut refused,
            vec![
                join_as(3, "a", "T1"),
                subscribe(4, "b", "T1", "main", O_SOCK),
                subscribe(5, "a", "T1", "develop", O_SOCK),
            ],
        );
        assert!(
            matches!(
                refused.subscriptions[&sid("a")].state,
                SubscriptionState::Refused(_)
            ),
            "T1 is bound to main"
        );
        assert!(
            refused.joins.contains_key(&sid("a")),
            "refused subscribe keeps the join"
        );
    }

    #[test]
    fn unsubscribe_finish() {
        let finish_as = |second: i64, token: &str| Event::UnsubscribeRequested {
            session: sid("o"),
            token: MessagingToken(token.to_owned()),
            finish: true,
            at: at(second),
        };
        let mut fleet = orchestrated();
        let _ = run(
            &mut fleet,
            vec![
                seeded(6),
                titled(12, "x", "CC - x - [WIP]", "/r1", None),
                join_as(7, "x", "T1"),
            ],
        );
        let wrong = apply(&mut fleet, finish_as(8, "nope"));
        assert!(records(&wrong).is_empty(), "wrong token: {wrong:?}");
        assert!(fleet.joins.contains_key(&sid("w")));

        let finished = apply(&mut fleet, finish_as(9, "t"));
        let left: Vec<Record> = records(&finished)
            .into_iter()
            .filter(|record| matches!(record, Record::Left { .. }))
            .collect();
        assert_eq!(
            left,
            vec![
                left_record("w", "the fleet finished"),
                left_record("x", "the fleet finished"),
            ]
        );
        assert_eq!(archives(&finished).len(), 1, "the run dissolves");
        assert!(!fleet.instances.contains_key(&fid("T1")));

        let mut plain = orchestrated();
        let _ = apply(&mut plain, seeded(6));
        let _ = apply(&mut plain, unsubscribe_as(7, "o"));
        assert!(
            plain.joins.contains_key(&sid("w")),
            "plain unsubscribe keeps members"
        );
    }

    #[test]
    fn replay_rebuilds_instances() {
        let legacy = || {
            vec![
                Event::Joined {
                    session: sid("w"),
                    fleet: fid("T1"),
                    token: MessagingToken("t".to_owned()),
                    source: JoinSource::Replay {
                        member: MemberId("A".to_owned()),
                    },
                    at: at(1),
                },
                journal_as(2, "o", "t", "note"),
            ]
        };
        let (mut one, mut two) = (repo_fleet(&["/r1"]), repo_fleet(&["/r1"]));
        let _ = run(&mut one, legacy());
        let _ = run(&mut two, legacy());
        let derived = one.instances[&fid("T1")].id;
        assert_eq!(
            derived,
            two.instances[&fid("T1")].id,
            "same id across replays"
        );
        assert_eq!(derived, InstanceId::legacy(&fid("T1"), at(1)));

        let recorded = candidate(9, "T1");
        let mut started = repo_fleet(&["/r1"]);
        let _ = run(
            &mut started,
            vec![
                Event::FleetStarted {
                    fleet: fid("T1"),
                    instance: recorded,
                    at: at(9),
                },
                Event::Left {
                    session: sid("w"),
                    at: at(10),
                },
            ],
        );
        assert_eq!(started.instances[&fid("T1")].id, recorded, "recorded id");
        let _ = apply(
            &mut started,
            Event::Dissolved {
                fleet: fid("T1"),
                instance: recorded,
                at: at(11),
            },
        );
        assert!(!started.instances.contains_key(&fid("T1")), "dissolved");
        let _ = apply(
            &mut started,
            Event::Joined {
                session: sid("w"),
                fleet: fid("T1"),
                token: MessagingToken("t".to_owned()),
                source: JoinSource::Replay {
                    member: MemberId("A".to_owned()),
                },
                at: at(12),
            },
        );
        assert_eq!(
            started.instances[&fid("T1")].id,
            InstanceId::legacy(&fid("T1"), at(12)),
            "a later join starts another run"
        );
    }

    #[test]
    fn evicted_session_drops_its_ended_subscription() {
        let mut fleet = orchestrated();
        let _ = apply(&mut fleet, end_as("o", 10));
        assert!(
            fleet.subscriptions.contains_key(&sid("o")),
            "ended, still listed"
        );
        for n in 0..fleet.caps.max_terminal() {
            let other = format!("t{n:04}");
            let _ = apply(&mut fleet, start_as(&other, 20));
            let _ = apply(&mut fleet, end_as(&other, 21));
        }
        assert!(!fleet.sessions.contains_key(&sid("o")), "evicted");
        assert!(
            !fleet.subscriptions.contains_key(&sid("o")),
            "subscription gone"
        );
    }
}
