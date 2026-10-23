//! The peek edge (exploration D1): an `rmcp` Streamable HTTP client of the central peek, adopted by its `/healthz`
//! identity and never spawned. One tokio task. Spawn-tree requests (a subscriber session each) that arrive while a
//! round runs coalesce into the next round, so at most one round is in flight and one queued. Each round opens a
//! fresh MCP connection (as the Go client does per call group), calls `session_events` per session, closes,
//! forwards one `PeekObserved` per session, and reports the `peek` sensor: ok, or degraded with the failing step.
//! A degraded round is repeated after `retry` until one succeeds, so a peek restart recovers without a request.

use std::collections::BTreeSet;
use std::net::SocketAddr;
use std::path::{Path, PathBuf};
use std::time::Duration;

use jiff::Timestamp;
use rmcp::ServiceExt as _;
use rmcp::model::{CallToolRequestParams, ClientCapabilities, ClientConfig, Implementation};
use rmcp::service::{RoleClient, RunningService};
use rmcp::transport::StreamableHttpClientTransport;
use rmcp::transport::streamable_http_client::StreamableHttpClientTransportConfig;
use tokio::sync::mpsc;

use crate::edges::sensor;
use crate::engine::Input;
use crate::event::Event;
use crate::fleet::{SensorName, SensorState};
use crate::ids::SessionId;
use crate::peek::{Identity, SessionEvents, SpawnTree};
use crate::protocol::PEEK_SESSION_EVENTS_TOOL;

/// Where peek listens and whose Claude home it must serve.
#[derive(Debug, Clone)]
pub struct Options {
    /// peek's loopback address; MCP at `/mcp`, identity at `/healthz`.
    pub addr: SocketAddr,
    /// The Claude home this daemon serves.
    pub claude_home: PathBuf,
    /// How long a degraded round waits before it is repeated.
    pub retry: Duration,
    /// `/healthz` request timeout (`peek_healthz_timeout`).
    pub healthz_timeout: Duration,
    /// Initialize and per-call timeout (`peek_call_timeout`); peek's warm-up blocks a call for up to 2 minutes.
    pub call_timeout: Duration,
}

/// Starts the edge as a task on the current runtime (called from `serve::run`), draining `requests`.
pub fn start(
    inbox: mpsc::Sender<Input>,
    requests: mpsc::UnboundedReceiver<SessionId>,
    options: Options,
) {
    tokio::spawn(run(inbox, requests, options));
}

async fn run(
    inbox: mpsc::Sender<Input>,
    mut requests: mpsc::UnboundedReceiver<SessionId>,
    options: Options,
) {
    let http = match reqwest::Client::builder()
        .timeout(options.healthz_timeout)
        .build()
    {
        Ok(http) => http,
        Err(err) => {
            let detail = format!("http client: {err}");
            let _ = inbox
                .send(sensor(
                    SensorName::Peek,
                    SensorState::Degraded,
                    Some(detail),
                ))
                .await;
            return;
        }
    };
    let home =
        dunce::canonicalize(&options.claude_home).unwrap_or_else(|_| options.claude_home.clone());
    let mut round = BTreeSet::new();
    loop {
        let outcome = fetch(&http, options.addr, &home, &round, options.call_timeout).await;
        let is_degraded = outcome.is_err();
        let report = match outcome {
            Ok(trees) => {
                for tree in trees {
                    let observed = Input::Event(Event::PeekObserved {
                        tree,
                        at: Timestamp::now(),
                    });
                    if inbox.send(observed).await.is_err() {
                        return;
                    }
                }
                sensor(SensorName::Peek, SensorState::Ok, None)
            }
            Err(detail) => sensor(SensorName::Peek, SensorState::Degraded, Some(detail)),
        };
        if inbox.send(report).await.is_err() {
            return;
        }
        let next = if is_degraded {
            tokio::select! {
                received = requests.recv() => received.map(Some),
                () = tokio::time::sleep(options.retry) => Some(None),
            }
        } else {
            requests.recv().await.map(Some)
        };
        match next {
            None => return,
            Some(Some(first)) => round = BTreeSet::from([first]),
            Some(None) => {}
        }
        while let Ok(next) = requests.try_recv() {
            round.insert(next);
        }
    }
}

/// One round: identity, connect, `session_events` per session of `round`, close. `Err` is the sensor detail.
async fn fetch(
    http: &reqwest::Client,
    addr: SocketAddr,
    home: &Path,
    round: &BTreeSet<SessionId>,
    call_timeout: Duration,
) -> Result<Vec<SpawnTree>, String> {
    let identity: Identity = http
        .get(format!("http://{addr}/healthz"))
        .send()
        .await
        .map_err(|err| format!("peek unreachable on {addr}: {err}"))?
        .json()
        .await
        .map_err(|err| format!("peek /healthz on {addr}: {err}"))?;
    let theirs = dunce::canonicalize(&identity.claude_home)
        .unwrap_or_else(|_| PathBuf::from(&identity.claude_home));
    identity.adopt(home, &theirs)?;
    let transport = StreamableHttpClientTransport::from_config(
        StreamableHttpClientTransportConfig::with_uri(format!("http://{addr}/mcp")),
    );
    let client = ClientConfig::new(
        ClientCapabilities::default(),
        Implementation::new("heartbeat", env!("CARGO_PKG_VERSION")),
    );
    let mut service = tokio::time::timeout(call_timeout, client.serve(transport))
        .await
        .map_err(|_| "peek initialize timed out".to_owned())?
        .map_err(|err| format!("peek initialize: {err}"))?;
    let observed = calls(&service, round, call_timeout).await;
    let _ = service.close().await;
    observed
}

async fn calls(
    service: &RunningService<RoleClient, ClientConfig>,
    round: &BTreeSet<SessionId>,
    call_timeout: Duration,
) -> Result<Vec<SpawnTree>, String> {
    let mut trees = Vec::with_capacity(round.len());
    for session in round {
        trees.push(
            session_events(service, session, call_timeout)
                .await?
                .spawn_tree(session.clone()),
        );
    }
    Ok(trees)
}

/// `session_events` with `json: true`, decoded from `structuredContent`; a tool error carries peek's own text
/// (e.g. the warm-up notice).
async fn session_events(
    service: &RunningService<RoleClient, ClientConfig>,
    session: &SessionId,
    call_timeout: Duration,
) -> Result<SessionEvents, String> {
    const TOOL: &str = PEEK_SESSION_EVENTS_TOOL;
    let mut arguments = serde_json::Map::new();
    arguments.insert("id".to_owned(), serde_json::Value::from(session.0.clone()));
    arguments.insert("json".to_owned(), serde_json::Value::Bool(true));
    let params = CallToolRequestParams::new(TOOL).with_arguments(arguments);
    let result = tokio::time::timeout(call_timeout, service.call_tool(params))
        .await
        .map_err(|_| format!("{TOOL} timed out"))?
        .map_err(|err| format!("{TOOL}: {err}"))?;
    if result.is_error == Some(true) {
        let text = result
            .content
            .first()
            .and_then(|block| block.as_text())
            .map(|text| text.text.clone())
            .unwrap_or_default();
        return Err(format!("{TOOL} failed: {text}"));
    }
    result
        .into_typed::<SessionEvents>()
        .map_err(|err| format!("{TOOL}: {err}"))
}
