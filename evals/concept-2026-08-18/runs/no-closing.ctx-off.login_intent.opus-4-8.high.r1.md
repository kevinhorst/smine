## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** concept skill (unattended)
> **Date:** 2026-08-18

---

## Goals

- Capture the destination a visitor was heading to *before* they authenticate, and land them there *after* — for both password login and SSO.
- Cut the ~40 tickets/month about "the link did not work" by honoring deep links (shared report URLs, invite emails, bookmarked dashboards) through the login boundary.
- Add no new persistent store: reuse the existing session cache.
- Leave the login form untouched and pass only an opaque intent id through the externally-owned SSO provider.
- Fail safe: an expired, missing, or invalid intent silently drops the user on the home screen, never an error.

---

## User Flows

### Deep link → password login → redirect

**Goals:**
- An anonymous visitor who opens a protected deep link and signs in with email + password lands on the original path.

**Options:**

**MVP**
- Capture the requested path when an unauthenticated request is redirected to `/login`. (~1d)
- Store the path server-side under a short-lived intent id in the session cache; hand the id back in a signed cookie. (~2d)
- On session issue, consume the intent, validate the stored path, and redirect there. (~2d)

**Backlog**
- Surface the pending destination on the login page ("You'll return to *Report 8813*"). (~2d)

**Challenges:**
- The intent must survive an unauthenticated → authenticated identity change without leaking the raw path to the browser or query string.
- An open redirect if a stored path is trusted without same-origin validation.

**Approach:**
- Path lives server-side only, keyed by an unguessable id; the id travels in a signed, http-only, same-site cookie.
- Validate on *consume*, not just on *create*, so a stale or tampered record cannot redirect off-origin.

---

### SSO round trip

**Goals:**
- The same deep-link-to-destination behavior when the visitor authenticates through the external SSO provider.

**Options:**

**MVP**
- Pass the intent id (never the path) through the provider's `state` parameter and reconcile it on callback. (~2d)
- Reconcile the `state` id against the signed cookie on return; consume and redirect identically to the password flow. (~1d)

**Backlog**
- Tolerate a dropped intent cookie on the SSO return by falling back to the `state` id alone (cross-cookie-loss resilience). (~2d)

**Challenges:**
- The SSO integration is owned by another team; we can only pass `state` through and read it back.
- The provider round trip may cross cookie/site boundaries that a same-site cookie could drop.

**Approach:**
- Treat `state` as the id carrier and the signed cookie as the corroborating channel; the id is opaque and single-use so passing it through `state` leaks nothing.
- Keep the consume/validate logic shared between both flows so SSO reuses password-flow validation verbatim.

---

### Expired or invalid intent

**Goals:**
- After the TTL, or when the stored path fails validation, land the user on the home screen with no error, and delete the intent record.

**Options:**

**MVP**
- On consume, if the intent is missing, expired, already consumed, or fails path validation, redirect to home and delete the record. (~1d)

**Challenges:**
- A silent failure must not mask a real bug; we still want signal.

**Approach:**
- Emit a metric/log on each drop reason (expired / invalid / missing / replayed) without showing the user anything.

---

## Decisions / Open Questions

**Decisions:**
- Reuse the existing session cache as the intent store; no new persistent store (constraint).
- Intent id travels in a signed cookie only, never the query string (brief).
- `state` carries only the intent id, never the path (brief).
- Path validation runs on consume: same-origin absolute-path only; absolute URLs and protocol-relative URLs (`//host`) rejected; max 2 KB (brief).
- One open intent per browser; creating a new one evicts the oldest (brief).
- Silent fallback to home on any expiry/validation failure, with the record deleted (brief).

**Open Questions:**
1. The brief's "one open intent per browser, oldest evicted" is stated as a per-browser cap, but the store key is a cookie the browser may not yet hold on first render. Assumption: "per browser" = per signed-cookie identity; a browser with no cookie is treated as having zero open intents. Confirm this is the intended granularity rather than per-IP or per-user.
2. Cookie attributes for the SSO round trip: a `SameSite=Lax` cookie survives a top-level redirect back from the provider, but `SameSite=Strict` would not. Assumption: `SameSite=Lax`, `HttpOnly`, `Secure`. Confirm against the SSO provider's return method (top-level GET vs. POST) — a POST callback needs `SameSite=None; Secure`.
3. TTL is 15 min from `created_at`; the SSO round trip consumes part of that budget. Assumption: 15 min is sufficient headroom for the provider round trip. Confirm no slow-IdP edge cases push past it.
4. Signing key for the intent-id cookie: assumption is the existing session-cookie signing secret is reused. Confirm whether a separate key is wanted for rotation isolation.
5. Behavior for an *already-authenticated* returning user who opens a deep link: they never hit `/login`, so no intent is created and they land directly on the deep link. Assumption: no intent machinery involved for this actor. Confirm nothing extra is expected here.
6. Non-GET deep links (e.g. a deep link that resolves to a form POST): assumption is only GET destinations are captured; a captured intent always redirects with GET. Confirm non-GET destinations are out of scope.


## file: plans/login_intent/concept/deep_link_password_login.md

# Deep Link → Password Login → Redirect

Detailed design for the password-login intent flow: the intent model, the two API calls (create on login-page render, consume on session issue), the validation rules, and the limits.

---

## Flows

### Capture and honor a deep link through password login

1. Visitor opens `/reports/8813` without a session.
2. Backend
   1. Auth middleware detects no valid session and prepares a redirect to `/login`.
   2. It captures the originally-requested path `/reports/8813` as the candidate intent path.
3. Backend — **create intent** (on login-page render)
   1. Validate the candidate path against the validation rules (below). On failure, render `/login` with no intent.
   2. Enforce the per-browser limit: if the browser already presents a valid intent cookie, evict that (oldest) intent record before creating the new one.
   3. Mint an unguessable `id`, store `{id, path, created_at, consumed_at=null}` in the session cache under the intent key with a 15-minute TTL.
   4. Set a signed, `HttpOnly`, `Secure`, `SameSite=Lax` cookie carrying `id` only. Render `/login` (form unchanged).
4. Visitor submits email + password on the unchanged login form.
5. Backend — **consume intent** (on session issue)
   1. Auth service validates credentials and issues the session.
   2. Read `id` from the signed intent cookie. If absent or signature-invalid, redirect home.
   3. Load the intent record by `id`. If missing, expired, or `consumed_at` already set, redirect home and delete the record.
   4. Re-validate `path` against the validation rules. On failure, redirect home and delete the record.
   5. Mark `consumed_at`, delete the record (single-use), clear the intent cookie.
   6. Redirect to `path` (`/reports/8813`).
6. Visitor lands on `/reports/8813`.

---

## Security Considerations

- **Open redirect** — a stored path could send the user off-origin. Mitigation: same-origin absolute-path-only validation, enforced on *consume* (not just create) so a tampered or stale record can't redirect off-domain.
- **Path disclosure** — the destination could leak via URL/query string. Mitigation: path is server-side only; the browser holds only the opaque id, in an `HttpOnly` cookie.
- **Cookie tampering / id guessing** — a forged id could point to another record. Mitigation: cookie is signed with the server secret; id is an unguessable high-entropy token; records are single-use.
- **Replay** — reusing a consumed id. Mitigation: `consumed_at` set and record deleted on first consume; a re-presented id loads nothing and falls back to home.
- **State accumulation / DoS** — unbounded intent records. Mitigation: 15-min TTL plus one-open-intent-per-browser eviction cap.
- **SSO `state` leakage** — only the opaque id rides in `state`, never the path (see the SSO flow); a leaked `state` reveals nothing and is single-use.

---

## Limits

- **TTL**: 15 minutes from `created_at` (short-lived; covers a normal login, including an SSO round trip).
- **Max path length**: 2 KB (bounds cache footprint and rejects pathological inputs).
- **Path shape**: same-origin absolute path only — must start with a single `/`, must not start with `//` (protocol-relative), must not be an absolute URL with a scheme (prevents open redirect).
- **Open intents per browser**: 1 — creating a new intent evicts the oldest (bounds per-browser state).
- **Single use**: an intent is consumed exactly once, then deleted.
- **Method**: only GET destinations are captured; consume always redirects with GET.

---

## Models

### Intent

**Public:**
- id: unguessable high-entropy token; the value carried in the signed cookie and SSO `state` (string)
- path: the same-origin destination path to redirect to after auth (string)

**Internal / Not Exported:**
- created_at: mint time; TTL is measured from here (timestamp)
- consumed_at: set on first consume, then the record is deleted; guards replay (timestamp, nullable)

**Storage:**
- Session cache entry keyed by `intent:{id}` with a 15-minute expiry (no new persistent store).

**Unique Index:**
- id

---

## APIs

Both are internal server operations, not public REST endpoints — invoked inline by the auth middleware. Documented here as request/response contracts.

### POST create-intent (on login-page render)

Captures the pre-auth destination and issues the intent cookie.

**Notes:**
- Called by the auth middleware when redirecting an unauthenticated request to `/login`.
- Enforces the one-open-intent-per-browser cap by evicting the incoming cookie's intent before creating a new one.
- Silently skips creation (renders `/login` with no cookie) when the candidate path fails validation.

**Request fields:**
- candidate_path: the originally-requested path to capture (string)

**Request headers:**
- Cookie: existing signed intent cookie, if any (used for eviction)

**Response fields:**
- (none in body) — response is the `/login` render plus a `Set-Cookie` for the signed intent id

**Response headers:**
- Set-Cookie: signed, `HttpOnly`, `Secure`, `SameSite=Lax` intent-id cookie

**Rate Limits:**
Bounded implicitly by one-open-intent-per-browser; no separate limit.

**Example (200):**

Request:
```json
{ "candidate_path": "/reports/8813" }
```

Response:
```json
{ "set_cookie": "intent=<signed-id>; HttpOnly; Secure; SameSite=Lax", "render": "/login" }
```

### POST consume-intent (on session issue)

Resolves the intent after successful authentication and returns the redirect target.

**Notes:**
- Called by the auth service immediately after a session is issued (password or SSO).
- Validates the stored path again before redirecting; any failure returns the home path and deletes the record.
- Single-use: the record is deleted and the cookie cleared regardless of success or fallback.

**Request fields:**
- id: intent id, from the signed cookie (password flow) or the SSO `state` reconciled against the cookie (string)

**Request headers:**
- Cookie: signed intent cookie

**Response fields:**
- redirect_to: the validated destination path, or the home path on any failure (string)

**Response headers:**
- Set-Cookie: cleared intent cookie

**Rate Limits:**
None beyond the enclosing login rate limit.

**Example (200) — success:**

Request:
```json
{ "id": "<signed-id>" }
```

Response:
```json
{ "redirect_to": "/reports/8813", "set_cookie": "intent=; Max-Age=0" }
```

**Example (200) — expired/invalid fallback:**

Request:
```json
{ "id": "<expired-id>" }
```

Response:
```json
{ "redirect_to": "/", "set_cookie": "intent=; Max-Age=0" }
```

---

## Validation Rules

Applied on both create and consume; consume is authoritative.

- Non-empty; length ≤ 2 KB.
- Starts with exactly one `/` (reject `//host` protocol-relative and bare relative paths).
- No scheme — reject anything parseable as an absolute URL (`http:`, `https:`, `mailto:`, etc.).
- Same-origin: after parsing, host/scheme components must be empty (path-only).
- On any failure: no error to the user; redirect home; delete any stored record.

---

## Long-Tail Tasks

### Observability

- Emit a metric per consume outcome: `success`, `expired`, `invalid_path`, `missing`, `replayed`.

### Configuration

- TTL (15 min), max path length (2 KB), and cookie attributes as config, not hard-coded — open question 2 (SSO cookie `SameSite`) may force `SameSite=None; Secure`.

### Shared consume path

- Ensure password and SSO flows call the identical consume/validate routine so validation can't drift between them.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep link → password login

**As an anonymous visitor** who opened a shared report link, I want to sign in with my email and password and land on that exact report, so that I don't have to find the link again after logging in.

**As an anonymous visitor**, I want my intended destination remembered without it appearing in the URL bar, so that the link I was sent isn't exposed or copied by accident.

---

## SSO round trip

**As an anonymous visitor** who opened a deep link and authenticates through my company's SSO, I want to land on the original destination after the provider sends me back, so that single sign-on works the same as password login.

**As a security reviewer**, I want only an opaque, single-use id to travel through the external SSO provider, so that the actual destination path is never handed to a third party.

---

## Safe fallback

**As a visitor** whose saved destination has expired or become invalid, I want to simply land on the home screen without an error message, so that a stale link doesn't dump a confusing error in front of me.

**As an on-call engineer**, I want each dropped-intent case to emit a reason in logs/metrics, so that a silent user fallback still gives me signal when something is actually broken.

---

## Guardrails

**As a security reviewer**, I want stored destinations validated to same-origin paths only on consume, so that a captured intent can never be turned into an open redirect off our domain.

**As a platform owner**, I want intent records to live in the existing session cache with a 15-minute TTL and one open intent per browser, so that the feature adds no new datastore and can't accumulate unbounded state.


