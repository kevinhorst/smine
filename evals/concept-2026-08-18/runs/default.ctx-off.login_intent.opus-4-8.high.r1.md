## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** Concept skill (unattended)
> **Date:** 2026-08-18

---

## Goals

- Capture the destination a visitor intended to reach before they authenticate, and honor it after sign-in — for both password login and SSO.
- Send a visitor who opened `/reports/8813` while signed out back to `/reports/8813` after they sign in, not the generic home screen.
- Cut the ~40 tickets/month about "the link did not work" by preserving deep-link destinations across the login boundary.
- Never leak the destination path through a URL query string; carry only an opaque intent id in transit.
- Reuse the existing session cache — no new persistent store, no change to the login form.

---

## User Flows

### Deep link → password login → redirect

**Goals:**
- An anonymous visitor who lands on a protected deep link is returned to that exact path after email + password login.

**Options:**

**MVP**
- On login-page render for an unauthenticated deep-link request, create a server-side intent record keyed by a short-lived id; store the id in a signed cookie. (~2d)
- On session issue, consume the intent: validate the stored path, redirect there, delete the record. (~1.5d)
- Same-origin path validation, TTL expiry, and length cap enforced at both create and consume. (~1d)

**Backlog**
- Surface a subtle "returning you to {page}" hint on the login screen. (~1d)

**Challenges:**
- The intended path must never travel in the query string (referrer leakage, shoulder-surfing, log capture).
- The cookie must be tamper-evident so an attacker can't swap in another id.

**Approach:**
- Path lives only server-side against the intent id; the id travels in a signed (HMAC) cookie.
- Validate the path again at consume time — never trust the value stored at create time to still be safe.

### SSO round trip

**Goals:**
- The same capture-and-honor behavior when the visitor authenticates through the external SSO provider.

**Options:**

**MVP**
- Pass the intent id (only the id, never the path) through the provider's `state` parameter so it survives the provider redirect. (~1.5d)
- On the SSO callback that issues the session, consume the intent exactly as the password flow does. (~1d)

**Backlog**
- Cross-check the `state` intent id against the signed cookie when both are present, to harden against login-CSRF. (~1d)

**Challenges:**
- The SSO integration is owned by another team; we can only pass `state` through, not change the provider handshake.
- `state` already serves as the SSO anti-CSRF nonce; overloading it with the intent id must not weaken that.

**Approach:**
- Treat the intent id as an additional opaque component of `state`; the id is unguessable and single-use, so it does not degrade the nonce.
- Keep the signed cookie as the primary carrier where the browser presents it; fall back to `state` across the provider hop.

### Expired or invalid intent

**Goals:**
- A visitor whose intent has expired or whose stored path fails validation lands on the home screen silently, with the record cleaned up.

**Options:**

**MVP**
- On consume, if the record is missing/expired or the path fails validation, redirect to `/` (home) with no error surfaced, and delete the record. (~0.5d)

**Backlog**
- Emit a metric/counter for expired-vs-invalid outcomes to size the real-world miss rate. (~0.5d)

**Challenges:**
- Distinguishing "expired" from "tampered/invalid" without exposing either to the user.

**Approach:**
- Both paths converge on the same user-visible outcome (silent home redirect); the distinction is captured only in server-side metrics.

---

## Decisions / Open Questions

**Decisions:**
- Intent records live in the existing session cache (a TTL key-value store), not a new persistent store — satisfies the no-new-store constraint and gives free expiry.
- The intended path is stored server-side; only the opaque intent id crosses the wire, in a signed cookie (password flow) or the `state` parameter (SSO flow).
- Validation runs at both create and consume: same-origin absolute paths only (must start with a single `/`), absolute URLs and protocol-relative (`//host`) rejected, max 2 KB, TTL 15 minutes.
- One open intent per browser; creating a new one evicts the oldest for that browser.
- The login form itself is untouched — capture happens on page render, consume happens on session issue.
- Expired/invalid consume is silent: redirect home, delete the record, no user-facing error.

**Open Questions:**
1. When an already-authenticated user (valid session cookie) hits a deep link, do we bypass login entirely and go straight to the path? Assumed yes — intent capture only triggers for unauthenticated requests; a live session short-circuits to the destination.
2. Should the intent cookie be `SameSite=Lax` (survives the top-level SSO redirect back) or `Strict`? Assumed `Lax`, because `Strict` would drop the cookie on the return leg of the SSO provider redirect. The `state`-carried id is the fallback either way.
3. Do we honor deep links to *non-protected* paths (public pages) too, or only to auth-gated destinations? Assumed only requests that were redirected to `/login` create intents; directly reachable pages never route through login and so never need an intent.
4. Is a `POST`/mutation deep link ever a valid destination, or only idempotent `GET` paths? Assumed `GET`-only — we redirect with a `GET`, so any captured path is replayed as a navigation, not a form submission.
5. What is the eviction key for "one open intent per browser" when no signed cookie yet exists (first visit)? Assumed the intent id in the freshly-set signed cookie is the browser identity; a second login-page render before consume overwrites the cookie and evicts the prior record.
6. On successful consume, is the intent cookie cleared immediately (single-use) or left to TTL? Assumed cleared immediately on consume to prevent replay.


## file: plans/login_intent/concept/deep_link_redirect.md

# Deep Link → Password Login → Redirect

The intent-capture implementation block for flow 1. Establishes the intent model, the create/consume API pair, the validation rules, and the per-browser limits that the SSO flow reuses.

**Estimated total: ~5.5–7d**

---

## Flows

### Anonymous visitor follows a deep link and signs in

1. Visitor opens `/reports/8813` with no session cookie.
2. Backend
   1. Auth gate finds no valid session, prepares a redirect to `/login`.
   2. Capture the originally requested path (`/reports/8813`) as the intent candidate.
3. Backend renders the login page (create-intent call):
   1. Validate the candidate path (see Validation Rules). If it fails, render `/login` with no intent and stop.
   2. Enforce the per-browser limit: if a prior intent id is presented in the signed cookie, evict that record.
   3. Create an intent record `{id, path, created_at, consumed_at=null}` in the session cache with a 15-minute TTL.
   4. Set the intent id in a signed cookie on the login response. The path is never written to the response.
4. Visitor submits email + password on the unchanged login form.
5. Backend authenticates and issues the session (consume-intent call):
   1. Read the intent id from the signed cookie; verify the signature. If absent/invalid, redirect to `/` (home).
   2. Load the intent record by id. If missing or expired, redirect to `/` and clear the cookie.
   3. Re-validate the stored path. If it fails now, delete the record, clear the cookie, redirect to `/`.
   4. Mark the record consumed (`consumed_at=now`), delete it (single-use), clear the intent cookie.
   5. Issue the session cookie and redirect to the stored path (`/reports/8813`).
6. Visitor lands on `/reports/8813`, now authenticated.

### Returning user with a live session follows a deep link

1. Visitor opens `/reports/8813` with a valid session cookie.
2. Backend
   1. Auth gate finds a valid session; no login redirect occurs.
   2. No intent is created.
3. Visitor lands on `/reports/8813` directly.

### Expired or invalid intent at consume time

1. Visitor signs in after the 15-minute TTL, or against a record whose path no longer validates.
2. Backend
   1. Consume-intent finds the record missing/expired, or re-validation fails.
   2. Delete the record if present, clear the intent cookie.
   3. Issue the session and redirect to `/` — no error surfaced.
3. Visitor lands on the home screen.

---

## Security Considerations

- **Open-redirect / off-site redirect** — a captured path could point off-origin, turning login into a redirector. Mitigation: same-origin path-only validation (must start with a single `/`, not `//`, no scheme/host), enforced at create *and* consume.
- **Path leakage via URL** — the destination in a query string leaks through referrers, browser history, and access logs. Mitigation: the path lives only server-side; only the opaque intent id travels, in a signed cookie / `state`.
- **Cookie tampering** — an attacker swapping the intent id could redirect a victim. Mitigation: HMAC-signed cookie; a bad signature is treated as no intent (home redirect).
- **Login CSRF** — an attacker planting an intent id to steer a victim post-login. Mitigation: single-use records, unguessable ids, immediate deletion on consume; SSO adds the `state`/cookie cross-check (backlog).
- **Cache exhaustion** — unbounded intent creation filling the session cache. Mitigation: one open intent per browser (oldest evicted) plus the 15-minute TTL bound total live records.
- **Stored-value trust** — a path safe at create time treated as safe forever. Mitigation: re-validate at consume; never redirect on the create-time verdict alone.

---

## Limits

- **TTL**: 15 minutes (short enough to bound cache footprint and replay window; long enough to cover a real login).
- **Max path length**: 2 KB (rejects oversized/abusive paths; comfortably above real deep links).
- **Open intents per browser**: 1 (oldest evicted on new create; prevents accumulation).
- **Allowed path shape**: same-origin absolute path only — starts with a single `/`, no scheme, no host, no protocol-relative `//` prefix.
- **Methods honored**: idempotent `GET` navigations only; the redirect replays as a `GET`.
- **Cookie**: signed (HMAC), `HttpOnly`, `Secure`, `SameSite=Lax`, single-use (cleared on consume).

---

## Models

### Intent

**Public:**
- id: opaque, unguessable identifier carried in the signed cookie / SSO `state` (string)
- path: the captured same-origin destination path (string)

**Internal / Not Exported:**
- created_at: creation timestamp; TTL is measured from here (timestamp)
- consumed_at: set when the intent is honored, immediately before deletion; null while open (timestamp, nullable)

**Storage:**
- Session cache (existing TTL key-value store), key = intent id, 15-minute expiry. No new persistent store.

**Unique Index:**
- id

---

## APIs

### POST /internal/login-intent

Create an intent when the login page is rendered for an unauthenticated deep-link request. Server-internal, invoked during login-page render — not a public browser-callable endpoint.

**Notes:**
- Called only when the auth gate redirected a deep link to `/login`.
- Validates the path; on failure returns no intent and the login page renders without one.
- Evicts the caller's prior intent (per-browser limit) before creating.
- Sets the intent id in a signed cookie; never returns the path in the body.

**Request fields:**
- path: the captured destination path (string)

**Request headers:**
- Cookie: existing signed intent cookie, if any (used for eviction)

**Response fields:**
- (none in body — the intent id is delivered via a `Set-Cookie` signed cookie)

**Response headers:**
- Set-Cookie: signed, `HttpOnly`, `Secure`, `SameSite=Lax` intent cookie

**Rate Limits:**
- Bounded implicitly by one-open-intent-per-browser plus the 15-minute TTL.

**Example (204):**

Request:
```json
{ "path": "/reports/8813" }
```

Response:
```json
{}
```

### POST /internal/login-intent/consume

Consume the intent at session issue, returning the redirect target. Server-internal, invoked inside the session-issue path for both password and SSO logins.

**Notes:**
- Reads the intent id from the signed cookie (password flow) or the SSO `state` (SSO flow).
- Missing, expired, or re-validation-failed records yield the home path `/` with no error.
- On success, marks `consumed_at`, deletes the record (single-use), and clears the intent cookie.

**Request fields:**
- intent_id: the id read from the signed cookie or SSO `state` (string)

**Request headers:**
- Cookie: signed intent cookie (password flow)

**Response fields:**
- redirect_to: the validated destination path, or `/` on any miss (string)

**Response headers:**
- Set-Cookie: cleared intent cookie

**Rate Limits:**
- One consume per login attempt.

**Example (200) — hit:**

Request:
```json
{ "intent_id": "k7Qm2f9aXb" }
```

Response:
```json
{ "redirect_to": "/reports/8813" }
```

**Example (200) — expired/invalid:**

Request:
```json
{ "intent_id": "k7Qm2f9aXb" }
```

Response:
```json
{ "redirect_to": "/" }
```

---

## Validation Rules

Applied at both create and consume:

- Must be a non-empty string of length ≤ 2 KB.
- Must begin with a single `/` — reject anything starting with `//` (protocol-relative) or containing a scheme (`http:`, `https:`, `mailto:` etc.).
- Must be same-origin: no host component, no `@`, no backslashes (which some browsers normalize to `/`).
- Must decode to a well-formed path; reject on decode error.
- On any failure: no intent created (create) or home redirect + record deleted (consume).

---

## Long-Tail Tasks

### Metrics

- Counter for consume outcomes: hit / expired / invalid / no-cookie — sizes the real-world miss rate and validates the 15-minute TTL.

### Hardening (backlog)

- SSO `state` ↔ cookie intent-id cross-check for login-CSRF resistance.
- Optional per-IP create-rate ceiling if metrics show cache pressure.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep-link return

**As an anonymous visitor** who opened a shared report link while signed out, I want to land on that exact report after I sign in, so that I don't have to hunt for it again from the home screen.

**As an anonymous visitor** arriving from an invite email or a bookmarked dashboard, I want my intended page remembered across the login step, so that the link "just works".

---

## SSO parity

**As a visitor who signs in through SSO**, I want my intended destination honored after the provider round trip, so that SSO users get the same deep-link behavior as password users.

---

## Silent recovery

**As a visitor whose link sat too long before I signed in**, I want to land on the home screen without an error, so that an expired link never blocks me or shows a confusing message.

**As a visitor who followed a malformed or off-site link**, I want to be quietly taken to the home screen, so that a bad destination can't redirect me somewhere unsafe.

---

## Privacy

**As a security-conscious user**, I want my intended path kept out of the URL and out of server logs, so that a shared or captured login URL never reveals where I was headed.

---

## Support reduction

**As a support agent**, I want deep links to survive login, so that the recurring "the link did not work" tickets stop arriving.


