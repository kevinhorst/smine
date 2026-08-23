## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** Kevin Horst
> **Date:** 2026-08-18

---

## Goals

- Capture a visitor's intended destination *before* authentication and honor it *after* sign-in, for both password login and SSO.
- Cut the ~40 support tickets/month about deep links "not working" by landing users on the page they clicked, not the generic home screen.
- Carry the intent safely: server-side path storage keyed by an opaque id, id in a signed cookie, never the path in a query string.
- Reuse the existing session cache — no new persistent store, no change to the login form.

---

## User Flows

### Deep link → password login → redirect

**Goals:**
- An anonymous visitor who opens `/reports/8813`, gets bounced to `/login`, and signs in with email + password lands back on `/reports/8813`.

**Options:**

**MVP**
- Store the intended path server-side against a short-lived intent id on login-page render. (~2d)
- Issue the intent id in a signed, same-origin, HttpOnly cookie. (~1d)
- On session issue, consume the intent: validate the stored path, redirect, delete the record. (~2d)
- Validation: same-origin path only, ≤2 KB, reject absolute and protocol-relative URLs. (~1d)

**Backlog**
- Preserve intent through a "create account" detour off the login page. (~2d)

**Challenges:**
- The path must never be attacker-controllable into an open redirect.

**Approach:**
- Only ever store/redirect to a path that starts with a single `/` and is not `//`; reject anything with a scheme or host. Redirect target is derived from the *stored* record, not from any request input at consume time.

---

### SSO round trip

**Goals:**
- The same deep-link → destination behavior when the visitor authenticates through the external SSO provider.

**Options:**

**MVP**
- Pass the intent id (only the id) through the SSO provider's `state` parameter so it survives the provider redirect. (~2d)
- On callback + session issue, resolve the intent id from `state`, then consume exactly as the password flow does. (~1d)

**Backlog**
- (none)

**Challenges:**
- The SSO integration is owned by another team; we can only pass `state` through, not change the provider handshake.
- The signed cookie may not be present on the provider's callback request depending on redirect chain and `SameSite`.

**Approach:**
- Treat `state` as the authoritative intent-id carrier for SSO; the cookie is the carrier for password login. On consume, accept the id from whichever channel the flow used. `state` carries only the opaque id — never the path — so nothing sensitive leaves our origin.

---

### Expired or invalid intent

**Goals:**
- A visitor whose intent has expired or whose stored path fails validation lands on the home screen silently.

**Options:**

**MVP**
- On consume, if the record is missing/expired or the path fails validation: redirect to home, show no error, delete the record. (~1d)

**Backlog**
- (none)

**Challenges:**
- Must not leak "your link expired" as an error state — the brief calls for a silent fallback.

**Approach:**
- Fallback to home is the default branch of consume; expiry is handled by the cache TTL plus an explicit validation re-check at consume time (defense in depth).

---

## Decisions / Open Questions

**Decisions:**
- Intent path is stored server-side in the existing session cache; only the opaque id travels to the client (signed cookie) or through SSO (`state`). Rationale: keeps the path off the wire and non-tamperable.
- One open intent per browser; creating a new intent evicts the previous one. Rationale: bounds cache use and matches "oldest evicted" from the brief.
- Validation runs twice — at create (reject bad paths early) and at consume (defense against a tampered or stale record). Rationale: cheap, closes the open-redirect window.

**Open Questions:**
1. **(assumed: id from cookie, fall back to `state`)** When both the signed cookie *and* an SSO `state` id are present on the same request, which wins? Assumption: prefer `state` for SSO callbacks, cookie otherwise; they should reference the same record in practice.
2. **(assumed: no)** Should a *returning* user with a live session who hits a deep link ever touch the intent machinery at all, or bypass it entirely and go straight to the destination? Assumption: returning users bypass — intent is only created on the `/login` render.
3. **(assumed: yes, silently)** Should query-string and fragment parts of the deep link (`/reports/8813?tab=raw#row-2`) be preserved? Assumption: preserve query string within the 2 KB limit; fragments never reach the server so they cannot be captured.
4. **(assumed: session cache namespace `intent:`)** Exact key namespace and eviction interaction with existing session-cache entries — confirm no key collision with session records.
5. **(assumed: reject)** Are same-origin paths that resolve to auth endpoints themselves (`/login`, `/logout`) allowed as destinations, or should they be blocked to avoid redirect loops? Assumption: block `/login` and `/logout` as destinations, fall back to home.


## file: plans/login_intent/concept/intent_lifecycle.md

# Intent Lifecycle (Deep link → password login → redirect)

Detailed design for flow 1: create an intent on login-page render, consume it on session issue.

---

## Flows

### Create intent on login-page render

1. Anonymous visitor opens `/reports/8813`.
2. The web app has no session cookie, so it captures the requested path and redirects the browser to `/login`.
3. Backend (login-page render)
   1. Validate the captured path (see **Validation**). If it fails, render `/login` with no intent and stop.
   2. Generate an opaque `id` (unguessable random, e.g. 128-bit).
   3. Evict any existing open intent bound to this browser's intent cookie, then write `{id, path, created_at, consumed_at=null}` to the session cache under `intent:{id}` with a 15-minute TTL.
   4. Set a signed, HttpOnly, `Secure`, `SameSite=Lax` cookie carrying only `id`.
4. Visitor submits email + password on the unchanged login form.

### Consume intent on session issue

1. Visitor's credentials are accepted; the auth service issues a session.
2. Backend (session issue)
   1. Read `id` from the signed intent cookie. If absent/tampered, redirect to home and stop.
   2. Look up `intent:{id}`. If missing (expired/evicted) or already `consumed_at`, redirect to home, delete the cookie, and stop.
   3. Re-validate the stored `path` (see **Validation**). If it fails, delete the record + cookie, redirect to home, stop.
   4. Set `consumed_at`, delete the record and the cookie (single-use).
   5. Redirect the browser to the stored `path`.
3. Visitor lands on `/reports/8813`.

### Expired or invalid intent

1. Visitor signs in after the TTL, or against a record whose path no longer validates.
2. Backend
   1. Lookup misses (TTL expiry) or re-validation fails.
   2. Delete any lingering record + cookie.
   3. Redirect to home with no error surfaced.

---

## Security Considerations

- **Open redirect** — the redirect target is read only from the server-stored record, never from a request parameter at consume time; validation rejects any path with a scheme, host, or leading `//`. Mitigates attacker-supplied off-site redirects.
- **Path tampering** — the path never leaves the server; the client holds only the opaque `id` in a signed cookie, so a client cannot substitute a different destination.
- **Intent forgery / fixation** — `id` is unguessable and the cookie is signed; a forged or lifted `id` that doesn't match a live record falls through to home.
- **Replay** — records are single-use (`consumed_at` set + deleted on consume), so a captured `id` can't be reused.
- **Cookie theft scope** — HttpOnly blocks script access; `Secure` keeps it off plaintext; `SameSite=Lax` still allows the top-level `/login` navigation while limiting cross-site sends.
- **Cache exhaustion** — one open intent per browser (oldest evicted) plus the 15-minute TTL bound the footprint in the shared session cache.

---

## Limits

- **TTL**: 15 minutes (short-lived; a stale deep link should fall back to home, not linger).
- **Max path length**: 2 KB (bounds cache entry size and blocks pathological URLs).
- **Open intents per browser**: 1 — creating a new intent evicts the previous one (oldest evicted).
- **Allowed path shape**: same-origin absolute path only — must start with a single `/`, must not start with `//`, must contain no scheme or host.
- **Single use**: an intent is consumed exactly once, then deleted.

---

## Models

### Intent

**Public:**
- id: opaque unguessable identifier, travels in the signed cookie / SSO `state` (string)

**Internal / Not Exported:**
- path: same-origin destination path to redirect to after auth (string, ≤2 KB)
- created_at: write time; TTL is measured from here (timestamp)
- consumed_at: set when the intent is redeemed; null while open (nullable timestamp)

**Unique Index:**
- id (cache key `intent:{id}`)

Stored in the existing session cache; no new persistent store.

---

## APIs

Both are server-internal steps in existing request handlers, not new public endpoints. Described as calls for clarity.

### Create intent (on `GET /login` render)

Creates an intent record for the captured pre-login path and binds it to the browser via a signed cookie.

**Notes:**
- Only invoked when a valid same-origin path was captured; otherwise no intent is created.
- Evicts any prior open intent for this browser before writing.
- Path is validated before the record is written.

**Request fields:**
- path: the captured pre-login destination (string)

**Response fields:**
- id: opaque intent id (string) — also emitted as the signed intent cookie

**Response headers:**
- `Set-Cookie`: signed, HttpOnly, Secure, SameSite=Lax cookie carrying `id`

**Rate Limits:**
- Bounded implicitly by one-open-intent-per-browser + TTL; no separate quota.

**Example (200):**

Request:
```json
{ "path": "/reports/8813?tab=raw" }
```

Response:
```json
{ "id": "b7c1f0e2a9d4487f" }
```

### Consume intent (on session issue)

Redeems the intent bound to the request and returns the redirect target.

**Notes:**
- Single-use: the record is deleted on success.
- Missing, expired, already-consumed, or invalid → returns the home path, no error.

**Request headers:**
- `Cookie`: signed intent cookie carrying `id` (password flow). For SSO, the id arrives via the provider `state` parameter instead.

**Response fields:**
- redirect: the validated stored path, or the home path on any failure (string)

**Rate Limits:**
- None beyond normal auth throttling.

**Example (200 — valid):**

Request:
```json
{ "id": "b7c1f0e2a9d4487f" }
```

Response:
```json
{ "redirect": "/reports/8813?tab=raw" }
```

**Example (200 — expired/invalid):**

Request:
```json
{ "id": "b7c1f0e2a9d4487f" }
```

Response:
```json
{ "redirect": "/" }
```

---

## Validation

Applied at create (reject early) and re-applied at consume (defense in depth):

- Length ≤ 2 KB, else reject.
- Must start with a single `/`.
- Must NOT start with `//` (protocol-relative URL).
- Must NOT contain a scheme (`http:`, `https:`, or any `scheme:` prefix).
- Must NOT encode a host (no `\`, no `@`-host tricks, no backslash-normalized `/\`).
- Destination must not be an auth endpoint (`/login`, `/logout`) — see Open Question 5.
- On any failure: no record is written (create) or the record is deleted and the user goes home (consume).

---

## Long-Tail Tasks

### SSO carry-through (flow 2, cross-block)

- Confirm the provider `state` round-trips the opaque `id` unchanged and within the provider's `state` size limit.
- Decide cookie-vs-`state` precedence when both are present (concept Open Question 1).

### Session cache

- Confirm `intent:` key namespace does not collide with existing session records (concept Open Question 4).
- Verify eviction of the prior open intent is atomic with writing the new one.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep-link sign-in

**As an anonymous visitor** who opens a shared report URL, I want to land on that exact report after I sign in with my email and password, so that the link I was given actually takes me where it promised.

**As an anonymous visitor** who was bounced to the login page, I want my intended destination remembered without it appearing in the address bar, so that I don't leak or accidentally share a URL that encodes where I was going.

---

## SSO sign-in

**As an anonymous visitor** using single sign-on, I want my intended destination to survive the trip out to the identity provider and back, so that federated login lands me on the same page a password login would.

---

## Silent fallback

**As a visitor** whose deep link sat unopened past the 15-minute window, I want to simply arrive on the home screen after signing in, so that I'm not blocked by an error about an expired link.

**As a visitor** who followed a malformed or off-site link, I want the app to quietly send me home, so that a bad destination never turns into a broken page or a redirect somewhere I didn't intend.

---

## Safety

**As the auth service owner**, I want the redirect target to only ever be a same-origin path drawn from a server-stored record, so that the login flow can't be turned into an open redirect.

**As a returning user** with a live session, I want a deep link to take me straight to its destination, so that I'm not routed through login machinery I don't need.


