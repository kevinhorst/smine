//! The CLI's side of the HTTP API: every call first checks `/healthz`, so a CLI never reads another profile's fleet
//! (concept D18). Blocking `ureq`, a `client_timeout` budget per request, typed errors: no daemon →
//! `Unreachable`, another profile → `ForeignHome`, non-2xx → `Refused`.

use std::net::SocketAddr;
use std::path::PathBuf;
use std::time::Duration;

use crate::status::Filter;
use crate::wire::{
    DoneAnswer, DoneBody, FindingRow, Health, JoinBody, JournalBody, LandAnswerRow, LandBody,
    LandEndBody, LandReleaseBody, LeaveBody, MemberRow, RecordedRow, RemoveBody, RuntimeActBody,
    RuntimeDocument, RuntimeSubscribeBody, RuntimeSubscriptionRow, RuntimeUnsubscribeBody,
    StatusDocument, SubscribeBody, SubscriptionRow, UnsubscribeBody, WorkerBody,
};
use crate::{Error, Result};

/// A client for one daemon address, on behalf of one claude home.
#[derive(Debug, Clone)]
pub struct Client {
    addr: SocketAddr,
    claude_home: PathBuf,
    agent: ureq::Agent,
}

impl Client {
    /// A client for `addr`, acting for `claude_home`, with `timeout` per request.
    #[must_use]
    pub fn new(addr: SocketAddr, claude_home: PathBuf, timeout: Duration) -> Client {
        let config = ureq::Agent::config_builder()
            .http_status_as_error(false)
            .timeout_global(Some(timeout))
            .build();
        Client {
            addr,
            claude_home,
            agent: ureq::Agent::new_with_config(config),
        }
    }

    /// `GET /healthz`; refused unless the daemon serves this client's claude home.
    pub fn verify(&self) -> Result<Health> {
        let health: Health = serde_json::from_str(&self.get("/healthz", &[])?)?;
        let expected =
            dunce::canonicalize(&self.claude_home).unwrap_or_else(|_| self.claude_home.clone());
        let served = dunce::canonicalize(&health.claude_home)
            .unwrap_or_else(|_| PathBuf::from(&health.claude_home));
        if expected != served {
            return Err(Error::ForeignHome {
                addr: self.addr,
                served: health.claude_home,
                expected: self.claude_home.display().to_string(),
            });
        }
        Ok(health)
    }

    /// `GET /status` for `filter`, after [`Client::verify`].
    pub fn status(&self, filter: &Filter) -> Result<StatusDocument> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.get("/status", &filter.query_pairs())?,
        )?)
    }

    /// `POST /subscribe`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn subscribe(&self, body: &SubscribeBody) -> Result<SubscriptionRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/subscribe", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /unsubscribe`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn unsubscribe(&self, body: &UnsubscribeBody) -> Result<SubscriptionRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/unsubscribe", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /join`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn join(&self, body: &JoinBody) -> Result<MemberRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/join", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /done`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn done(&self, body: &DoneBody) -> Result<DoneAnswer> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/done", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /land`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn land(&self, body: &LandBody) -> Result<LandAnswerRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/land", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /land/end`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn land_end(&self, body: &LandEndBody) -> Result<LandAnswerRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/land/end", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /land/release`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn land_release(&self, body: &LandReleaseBody) -> Result<LandAnswerRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/land/release", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /leave`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn leave(&self, body: &LeaveBody) -> Result<MemberRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/leave", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /remove`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn remove(&self, body: &RemoveBody) -> Result<MemberRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/remove", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /journal`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn record(&self, body: &JournalBody) -> Result<RecordedRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/journal", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `POST /worker`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn worker(&self, body: &WorkerBody) -> Result<MemberRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/worker", &serde_json::to_string(body)?)?,
        )?)
    }

    /// `GET /runtime`, after [`Client::verify`].
    pub fn runtime_status(&self) -> Result<RuntimeDocument> {
        self.verify()?;
        Ok(serde_json::from_str(&self.get("/runtime", &[])?)?)
    }

    /// `POST /runtime/subscribe`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's
    /// reason.
    pub fn runtime_subscribe(&self, body: &RuntimeSubscribeBody) -> Result<RuntimeSubscriptionRow> {
        self.verify()?;
        Ok(serde_json::from_str(&self.post(
            "/runtime/subscribe",
            &serde_json::to_string(body)?,
        )?)?)
    }

    /// `POST /runtime/unsubscribe`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's
    /// reason.
    pub fn runtime_unsubscribe(
        &self,
        body: &RuntimeUnsubscribeBody,
    ) -> Result<RuntimeSubscriptionRow> {
        self.verify()?;
        Ok(serde_json::from_str(&self.post(
            "/runtime/unsubscribe",
            &serde_json::to_string(body)?,
        )?)?)
    }

    /// `POST /runtime/act`, after [`Client::verify`]; a refusal is [`Error::Refused`] carrying the daemon's reason.
    pub fn runtime_act(&self, body: &RuntimeActBody) -> Result<FindingRow> {
        self.verify()?;
        Ok(serde_json::from_str(
            &self.post("/runtime/act", &serde_json::to_string(body)?)?,
        )?)
    }

    fn get(&self, path: &str, query: &[(&str, String)]) -> Result<String> {
        let mut request = self.agent.get(format!("http://{}{path}", self.addr));
        for (key, value) in query {
            request = request.query(*key, value);
        }
        self.answer(request.call())
    }

    fn post(&self, path: &str, body: &str) -> Result<String> {
        let request = self
            .agent
            .post(format!("http://{}{path}", self.addr))
            .header("content-type", "application/json");
        self.answer(request.send(body))
    }

    /// The body of a 2xx; a transport failure is `Unreachable`, any other status `Refused` with the body.
    fn answer(
        &self,
        sent: std::result::Result<ureq::http::Response<ureq::Body>, ureq::Error>,
    ) -> Result<String> {
        let unreachable = |detail: String| Error::Unreachable {
            addr: self.addr,
            detail,
        };
        let mut response = sent.map_err(|err| unreachable(err.to_string()))?;
        let status = response.status();
        let body = response
            .body_mut()
            .read_to_string()
            .map_err(|err| unreachable(err.to_string()))?;
        if status.is_success() {
            Ok(body)
        } else {
            Err(Error::Refused(format!("{status}: {body}")))
        }
    }
}
