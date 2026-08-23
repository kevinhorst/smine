## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** concept skill (unattended)
> **Date:** 2026-08-18

---

## Goals

- Preserve the destination a visitor arrived for (deep link) across the login boundary, so they land where they intended instead of the generic home screen.
- Support both password login and SSO with a single intent-capture mechanism.
- Eliminate the ~40 support tickets/month about "the link did not work".
- Keep the destination path server-side; never expose it in the query string or the SSO `state` parameter.
- Fail safe: an expired, missing, or invalid intent quietly falls back to the home screen — never an error page.

---

## User Flows

### Deep link → password login → redirect

**Goals:**
- An anonymous visitor who opens a protected deep link (e.g. `/reports/8813`) lands on that exact path after signing in with email + password.

**Options:**

**MVP**
- Capture the intended path when the login page renders for an unauthenticated request to a protected path. `~1d`
- Store the path server-side in the session cache under a short-lived intent id; hand the id back in a signed cookie. `~2d`
- On session issue, consume the intent, validate the stored path, and redirect there. `~1d`

**Backlog**
- Show a lightweight "returning you to X" confirmation on the destination page. `~1d`

**Challenges:**
- The login form itself must not change (constraint), so intent creation has to hang off the page render, not a new form field.
- Path validation must reject open-redirect vectors (absolute URLs, protocol-relative URLs, off-origin paths).

**Approach:**
- Create the intent during the login-page render path, before the form is served; the form markup is untouched.
- Validate on both write (create) and read (consume): same-origin, leading-slash relative path, ≤2 KB.

**Block total:** ~4–5d (detailed design: `deep_link_redirect.md`)

---

### SSO round trip

**Goals:**
- The same deep-link destination survives a full redirect out to the SSO provider and back.

**Options:**

**MVP**
- Carry the intent id — never the path — through the provider's `state` parameter. `~1d`
- On the SSO callback, read the intent id from `state`, cross-check it against the signed cookie, then consume as in the password flow. `~1–2d`

**Backlog**
- Degrade gracefully if the provider drops or truncates `state` (fall back to the cookie-only id). `~0.5d`

**Challenges:**
- The SSO provider integration is owned by another team; we can only pass `state` through it.
- `state` already serves CSRF protection for the SSO handshake; the intent id must coexist with that use, not replace it.

**Approach:**
- Treat `state` as an opaque carrier: encode the existing CSRF token plus the intent id. The path never leaves our server, so a leaked `state` reveals only an opaque id whose record expires in 15 minutes.
- Prefer the cookie as the authoritative id source and use `state` to bind the callback to the originating request.

**Block total:** ~2–3d

---

### Expired or invalid intent

**Goals:**
- After the TTL, or when the stored path fails validation at consume time, the user lands on the home screen with no error, and the intent record is deleted.

**Options:**

**MVP**
- On consume, if the intent is missing/expired, redirect to home silently. `~0.5d`
- On consume, re-validate the stored path; on failure, delete the record and redirect to home. `~0.5d`

**Backlog**
- Emit a metric (`intent.consumed`, `intent.expired`, `intent.invalid`) to confirm the ticket reduction. `~0.5d`

**Challenges:**
- "No error" must hold even when the record is structurally present but semantically stale.

**Approach:**
- Single consume codepath with a defined fallback: any negative outcome deletes the record (if present) and returns the home path.

**Block total:** ~1–1.5d

---

## Decisions / Open Questions

**Decisions:**
- No new persistent store: intents live in the existing session cache, keyed by intent id, with a 15-minute TTL enforced by the cache.
- Intent id travels only in a signed cookie (and, for SSO, echoed inside `state`); the path is never client-visible.
- Validation rules: same-origin relative path only (must start with `/`, must not start with `//`), no scheme, ≤2 KB. Validate on both create and consume.
- One open intent per browser; creating a new intent evicts the oldest for that browser.
- Fallback destination is the home screen for every failure mode, with no user-facing error.
- Login form markup is unchanged; intent creation attaches to the login-page render.

**Open Questions:**
1. What is the canonical "home screen" path used as the fallback — `/` or a named dashboard route? (Assumed `/` for now.)
2. Cookie attributes: assumed `HttpOnly`, `Secure`, `SameSite=Lax` (Lax is required so the cookie survives the top-level SSO redirect back to us). Confirm `SameSite=Lax` is acceptable given the SSO round trip.
3. "One open intent per browser" — is the browser identified by the intent cookie alone, or is there an existing anonymous browser/device id to key eviction on? (Assumed: keyed by the intent cookie; a browser with no cookie has zero open intents.)
4. Should intent capture fire for *any* unauthenticated hit to a protected path, or only when the request accepts HTML (to skip API/asset requests that 401)? (Assumed: only HTML navigations.)
5. Does the SSO `state` already carry a CSRF token we must preserve, and what is its size budget for co-encoding the intent id? (Assumed: yes, CSRF token present; intent id is a short opaque token that fits.)
6. Signing key for the cookie — reuse the existing session-cookie signing key/secret, or a dedicated one? (Assumed: reuse the existing session signing infrastructure.)
7. Is `consumed_at` needed at all given single-use consume deletes the record, or is it kept for a brief audit/idempotency window? (Assumed: kept to make double-consume idempotent within the TTL.)


## file: plans/login_intent/concept/deep_link_redirect.md

# Deep Link → Password Login → Redirect

Detailed design for Flow 1: capture the intended path at login-page render, honor it at session issue.

---

## Flows

### Deep link → login → redirect (happy path)

1. Anonymous visitor opens `/reports/8813`; the request has no session cookie.
2. Backend
   1. The route guard sees no session and the request accepts HTML; it triggers intent capture instead of a bare redirect.
   2. Validate the requested path: must start with a single `/`, must not start with `//`, must carry no scheme/host, must be ≤2 KB. On failure, skip capture (no intent created) and continue to step 3.
   3. Evict the visitor's existing open intent if one is referenced by the incoming intent cookie (one open intent per browser).
   4. Create an intent record in the session cache: generate `id`, store `{id, path, created_at, consumed_at: null}` with a 15-minute TTL.
   5. Issue a redirect to `/login` and set a signed cookie carrying only `id`.
3. Visitor is served the unchanged `/login` page and submits email + password.
4. Backend
   1. The auth service authenticates the credentials and issues a session.
   2. On session issue, read the intent `id` from the signed cookie; call consume-intent.
   3. Consume: look up the record. If missing/expired → redirect home (see `../concept.md` fallback). If present, re-validate `path` with the same rules; on failure, delete and redirect home.
   4. On success, mark `consumed_at`, delete the record (or let it lapse for idempotency within TTL), clear the intent cookie.
5. Visitor lands on `/reports/8813`.

### Returning user with an active session

1. Visitor with a valid session cookie opens `/reports/8813`.
2. Backend: the route guard sees a session; no intent is created and the page renders directly. No login boundary is crossed, so intent capture never fires.

---

## Security Considerations

- **Open redirect** — the stored path is validated on both create and consume: same-origin, leading single `/`, no scheme, no `//` prefix. Absolute URLs (`https://evil.com`) and protocol-relative URLs (`//evil.com`) are rejected, so a crafted deep link cannot bounce a user off-origin after login.
- **Path disclosure** — the path lives only server-side in the session cache. The client holds an opaque signed `id`, never the path; the query string and SSO `state` never carry the path.
- **Cookie tampering** — the intent cookie is signed with the existing session signing infrastructure; a tampered `id` fails signature verification and is treated as no intent (fallback home).
- **Fixation / replay** — single-use consume plus TTL: a captured `id` is valid for at most 15 minutes and is cleared at consume. A replayed `id` after consume finds no live record and falls back home.
- **Resource exhaustion** — one open intent per browser with oldest-evicted eviction bounds cache growth per visitor; the 2 KB path cap bounds per-record size; the TTL bounds lifetime.

---

## Limits

- **TTL**: 15 minutes (short-lived; a deep link followed later just lands home).
- **Max path length**: 2 KB (guards the cache entry and rejects pathological URLs).
- **Path scope**: same-origin relative only — must start with `/`, must not start with `//`, no scheme/host (open-redirect prevention).
- **Open intents per browser**: 1 (creating a new intent evicts the oldest for that browser).
- **Store**: existing session cache — no new persistent store; TTL enforced by the cache.

---

## Models

### Intent

**Public:**
- id: opaque identifier carried in the signed cookie and echoed in SSO `state` (string)

**Internal / Not Exported:**
- path: the captured same-origin destination path (string, ≤2 KB)
- created_at: creation timestamp; basis for the 15-minute TTL (timestamp)
- consumed_at: set when the intent is honored; null while open; enables idempotent double-consume within TTL (timestamp, nullable)

**Unique Index:**
- id

---

## APIs

Two internal calls. Neither is a public REST surface change; both hang off existing request handling (login-page render and session issue) so the login form is untouched.

### Create intent (on login-page render)

Invoked by the route guard when an unauthenticated HTML request hits a protected path.

**Notes:**
- Not a standalone client-called endpoint; fires server-side during the redirect-to-login step.
- Validates `path` before storing; invalid paths produce no intent (silent skip).
- Evicts the browser's prior open intent before creating the new one.
- Sets the signed intent cookie carrying only `id`.

**Request fields:**
- path: the requested destination path (string)

**Request headers:**
- Cookie: existing intent cookie, if any (used for eviction)

**Response fields:**
- id: the new intent id, returned via Set-Cookie (signed), not the body (string)

**Rate Limits:**
Bounded implicitly by one-open-intent-per-browser eviction; no separate limit.

**Example (302):**

Request:
```json
{ "path": "/reports/8813" }
```

Response:
```json
{ "id": "itn_9f3a1c", "set_cookie": "intent=<signed:itn_9f3a1c>; HttpOnly; Secure; SameSite=Lax", "location": "/login" }
```

### Consume intent (on session issue)

Invoked when the auth service issues a session (after password auth, or on SSO callback).

**Notes:**
- Reads `id` from the signed intent cookie (and, for SSO, cross-checks the `id` echoed in `state`).
- Missing/expired record → returns the home path, no error.
- Present record → re-validates `path`; on failure deletes the record and returns the home path.
- On success returns the stored `path`, marks `consumed_at`, deletes the record, clears the intent cookie.

**Request fields:**
- id: intent id from the signed cookie (string)

**Request headers:**
- Cookie: signed intent cookie

**Response fields:**
- redirect: the destination path to send the user to — the stored path on success, the home path on any fallback (string)

**Rate Limits:**
None beyond normal auth-endpoint limits.

**Example (200):**

Request:
```json
{ "id": "itn_9f3a1c" }
```

Response:
```json
{ "redirect": "/reports/8813" }
```

Example (expired/invalid → fallback):
```json
{ "redirect": "/" }
```

---

## Worker Tasks

- None. TTL expiry and eviction are handled by the session cache; no background sweeper is introduced.

---

## Infrastructure

- No new store or service. Reuses the existing session cache (TTL support required) and the existing cookie signing key/secret.
- Cookie attributes: `HttpOnly`, `Secure`, `SameSite=Lax` (Lax required so the cookie survives the top-level SSO redirect back to us — see `../concept.md` Open Question 2).

---

## Long-Tail Tasks

### Metrics

- Emit counters for `intent.created`, `intent.consumed`, `intent.expired`, `intent.invalid` to confirm the support-ticket reduction.

### Validation reuse

- Factor the same-origin path validator into one function used by both create and consume, so the two codepaths cannot drift.
  - Open question: is there an existing redirect/URL validator in the web app to reuse rather than write a new one?


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep-link return

**As an anonymous visitor** who opens a shared report link, I want to land on that report after I sign in with my email and password, so that I don't have to hunt for it from the home screen.

**As an anonymous visitor** who followed an invite email to a specific dashboard, I want that dashboard to open right after login, so that the invite delivers me where it promised.

---

## SSO parity

**As an anonymous visitor** whose company uses SSO, I want my deep-link destination preserved through the SSO provider round trip, so that single sign-on returns me to the same place password login would.

---

## Safe fallback

**As a visitor whose link sat unopened past the timeout**, I want to land on the home screen without an error when I finally sign in, so that a stale link never blocks me from getting in.

**As a visitor with a tampered or malformed intent**, I want the system to quietly send me home, so that a broken destination never exposes an error or an unexpected redirect.

---

## Security

**As the auth service**, I want the destination path kept server-side and off the URL and SSO `state`, so that no deep link can be turned into an open-redirect or leak a path to a third party.

**As a security reviewer**, I want same-origin-only path validation on both write and read, so that absolute and protocol-relative URLs can never be honored as a redirect target.

---

## Operations

**As a support lead**, I want the "the link did not work" tickets to drop, so that I can confirm the feature paid off with a measurable reduction.


