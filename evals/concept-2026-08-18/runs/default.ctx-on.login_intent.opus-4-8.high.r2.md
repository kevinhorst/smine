## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** concept skill (unattended)
> **Date:** 2026-08-18

---

## Goals

- Capture the destination a visitor intended before they hit `/login`, and return them there after they authenticate — for both password login and SSO.
- Cut the ~40 monthly "the link did not work" support tickets by making deep links survive the login round trip.
- Store the intent server-side, keyed by a short-lived id; never put the path in a query string or a client-readable cookie value.
- Fail closed and silent: an expired, missing, or invalid intent lands the user on the home screen with no error.
- Ship without a new persistent store (reuse the session cache) and without touching the login form itself.

---

## User Flows

### Deep link → password login → redirect

**Goals:**
- An anonymous visitor who opens `/reports/8813` and signs in with email + password lands back on `/reports/8813`.

**Options:**

**MVP**
- Create an intent record on login-page render: store the requested same-origin path against a generated intent id, set a signed cookie carrying the id. `~2d`
- Consume the intent on session issue: look up the path by id, validate it, redirect there, mark the record consumed and clear the cookie. `~1.5d`
- Same-origin path validation and the 2 KB length cap, applied both on create and on consume. `~1d`

**Backlog**
- Per-path allowlist / route existence check so a syntactically valid but dead path degrades to home. `~1d`

**Challenges:**
- The web app renders `/login` for any unauthenticated deep-link hit; the intent must be captured at that render, before the visitor types anything.
- An attacker could seed a victim's browser with an intent cookie pointing at an attacker-chosen path (login-redirect / open-redirect abuse).

**Approach:**
- Capture happens on the server render of `/login`: the origin path is read from the request (the path the visitor was denied), not from user input.
- The cookie carries only an opaque signed id; the path lives server-side and is re-validated on consume, so a tampered or replayed id cannot smuggle a foreign path.

---

### SSO round trip

**Goals:**
- The same deep-link-to-destination guarantee when the visitor authenticates through the external SSO provider.

**Options:**

**MVP**
- Pass the intent id through the SSO `state` parameter so it survives the provider redirect; on callback, consume the intent exactly as the password flow does. `~1.5d`

**Backlog**
- Reconcile a `state`-carried intent id with the cookie-carried one when both are present (defense in depth against `state` fixation). `~1d`

**Challenges:**
- The SSO integration is owned by another team; we only get to pass an opaque `state` value through it.
- The `state` parameter already does CSRF duty for the SSO handshake; overloading it must not weaken that.

**Approach:**
- Put only the intent id in `state`, never the path — the id is already opaque and server-validated, so it doubles as (or composes with) the existing CSRF nonce without leaking a destination.
- On callback, the intent is consumed server-side; the provider never sees or influences the redirect target.

---

### Expired or invalid intent

**Goals:**
- A stale or malformed intent never blocks or confuses the user.

**Options:**

**MVP**
- On consume, if the record is missing, past its 15-minute TTL, already consumed, or its stored path fails validation: redirect to home, delete the record, clear the cookie — no error surfaced. `~0.5d`

**Challenges:**
- A record can expire between create and consume (slow login, abandoned tab resumed later).
- A path valid at create time could be rejected at consume time if validation rules tighten.

**Approach:**
- Consume treats every failure mode identically — home screen, silent, record deleted — so there is a single, testable fallback rather than a matrix of error states.

---

## Decisions / Open Questions

**Decisions:**
- Intent lives in the existing session cache, not a new store — matches the "no new persistent store" constraint; the 15-minute TTL fits a cache-native expiry.
- The path never travels client-side; only a signed, opaque intent id does. Server-side lookup + re-validation is the single source of truth for the redirect target.
- Fallback is always the home screen, silent, with record deletion — one fallback path, not per-error messaging.
- One open intent per browser; creating a new one evicts the oldest. Bounds cache footprint and kills stale intents from abandoned tabs.

**Open Questions:**
1. What signs the intent-id cookie — the existing session-cookie signing key, or a dedicated key? (Assumed: reuse the session signing infrastructure; recorded as assumption, not yet confirmed.)
2. Does the SSO `state` parameter already carry a CSRF nonce we must compose with, or is it free for us to define? (Assumed: we own `state` shape for our leg and can encode `{csrf_nonce, intent_id}`; the other team owns pass-through only.)
3. Is "same-origin path" purely `^/` with no scheme/authority, or must we also reject paths that resolve to another app mount (e.g. `/admin` when the deep link came from the reporting app)? (Assumed: any `^/` same-origin path is honored; cross-mount restriction is Backlog.)
4. On consume for an authenticated user who lacks permission for the stored path, do we redirect there and let the app's own authz 403, or pre-check and fall to home? (Assumed: redirect and defer to the app's existing authorization; a pre-check is the Backlog allowlist item.)
5. Should a returning user with a live session who hits a deep link ever touch this flow, or bypass it entirely? (Assumed: with a valid session cookie there is no `/login` render, so no intent is created — the flow is anonymous-only by construction.)


## file: plans/login_intent/concept/deep_link_redirect.md

# Deep Link → Login → Redirect (Password Flow)

The password-login block: capture the intended path when `/login` renders, honor it when the session is issued. Estimate: **~4.5–5.5d** total (create + consume + validation, per the Overview MVP items).

---

## Flows

### Capture on login-page render

1. Anonymous visitor requests `/reports/8813`.
2. The web app finds no session and renders `/login`.
3. Backend
   1. Read the origin path from the denied request (server-side, not from any user-supplied field).
   2. Validate the path (see Validation Rules); if it fails, render `/login` with no intent and skip the rest.
   3. Evict this browser's existing open intent if one is present (see Limits).
   4. Create an intent record: new opaque `id`, the validated `path`, `created_at = now`, `consumed_at = null`; write it to the session cache with a 15-minute TTL.
   5. Set a signed cookie carrying only `id` (HttpOnly, Secure, SameSite=Lax, path-scoped to the auth routes).
4. Visitor sees the unchanged login form and enters email + password.

### Consume on session issue

1. Visitor submits valid credentials.
2. Backend
   1. The auth service issues the session as it does today.
   2. Read the intent `id` from the signed cookie; if absent or signature-invalid, redirect home and clear the cookie.
   3. Look up the record by `id` in the session cache.
   4. If the record is missing, expired (past TTL), or already `consumed`, redirect home and clear the cookie.
   5. Re-validate the stored `path` (see Validation Rules); on failure, delete the record, clear the cookie, redirect home.
   6. Set `consumed_at = now`, delete the record, clear the cookie.
   7. Issue a redirect to the stored `path`.
3. Visitor lands on `/reports/8813`.

---

## Security Considerations

- **Open redirect / login-redirect abuse** — the path is captured server-side from the denied request and re-validated (same-origin, no scheme, no authority) on both create and consume, so a caller cannot steer the redirect to an external or protocol-relative target.
- **Cookie tampering / id forgery** — the cookie carries only a signed, opaque id; an unsigned or altered id fails signature check and degrades to the home screen. The path itself is never client-side, so it cannot be tampered.
- **Replay** — the record is deleted and `consumed_at` stamped on first consume; a replayed cookie finds no record and falls to home.
- **Cache flooding** — one open intent per browser with oldest-evicted eviction bounds the per-browser footprint; the 15-minute TTL bounds lifetime for abandoned intents.
- **Path length abuse** — the 2 KB cap rejects oversized paths before they reach the cache.
- **Cookie exposure** — HttpOnly blocks script access; Secure forces TLS; SameSite=Lax limits cross-site submission while still allowing the top-level login navigation.

---

## Limits

- Intent TTL: **15 minutes** (short-lived; covers a normal login, expires abandoned tabs).
- Max path length: **2 KB** (rejects oversized/abusive paths; comfortably fits real deep links).
- Open intents per browser: **1** (creating a new intent evicts the oldest; bounds cache footprint).
- Allowed path shape: **same-origin, absolute-path only** (`^/`, no scheme, no authority, no protocol-relative `//`).
- Consumed record retention: **deleted immediately on consume** (no lingering record; `consumed_at` exists only for the in-flight decision).

---

## Validation Rules

Applied identically on **create** (login render) and **consume** (session issue). Any failure means: no redirect target — fall to the home screen (on consume, also delete the record and clear the cookie).

| Rule | Accept | Reject |
| --- | --- | --- |
| Must be a path, not a URL | `/reports/8813` | `https://evil.com/x`, `http://…` |
| No authority component | `/dashboard` | `//evil.com/x` (protocol-relative) |
| Absolute path only | `/reports/8813?tab=2` | `reports/8813`, `../admin` |
| Length | ≤ 2048 bytes | > 2048 bytes |
| Same origin | any `^/` path on this origin | any off-origin target |

Example — a captured value of `//evil.com/steal` is rejected because it carries an authority; the visitor lands home with no error.

---

## Models

### Intent

Stored in the session cache under the intent `id`, with a 15-minute expiry.

**Public:**
- id: opaque, unguessable identifier carried in the signed cookie and (for SSO) the `state` parameter (string)

**Internal / Not Exported:**
- path: the validated same-origin destination path (string, ≤ 2 KB)
- created_at: capture timestamp, basis for TTL expiry (timestamp)
- consumed_at: set when the intent is honored; null while open (nullable timestamp)

**Unique Index:**
- id (cache key)
- browser binding (the signed cookie ↔ id pairing enforces one open intent per browser)

---

## APIs

Both calls are internal to the auth/web boundary — they are not public endpoints and take no client-supplied path. They are described as the two server operations the brief calls out.

### Create intent (on `/login` render)

Invoked server-side when the app renders `/login` for a denied deep-link request.

**Notes:**
- The path is read from the denied request, never from a request body or query string.
- Runs the full validation rule set; on failure, no intent is created and no cookie is set.
- Evicts this browser's prior open intent before creating the new one.

**Inputs (server-derived):**
- origin_path: the path the visitor was denied (string)
- browser: identified via existing request context / prior intent cookie

**Effects:**
- Writes an Intent record to the session cache with a 15-minute TTL.
- Sets the signed intent-id cookie (HttpOnly, Secure, SameSite=Lax).

**Example — record written:**

```json
{
  "id": "3f9c2a7e-1b44-4e0a-9c2d-8e5f1a6b0d33",
  "path": "/reports/8813",
  "created_at": "2026-08-18T14:02:11Z",
  "consumed_at": null
}
```

### Consume intent (on session issue)

Invoked server-side immediately after the auth service issues a session (password or SSO callback).

**Notes:**
- Reads the intent id from the signed cookie (password flow) or the `state` parameter (SSO flow).
- Every failure mode — missing, expired, consumed, invalid path — resolves to the home screen, record deleted, cookie cleared.
- Deletes the record and stamps `consumed_at` on success; single-use.

**Inputs (server-derived):**
- id: from the signed cookie or SSO `state` (string)

**Effects:**
- On success: stamps `consumed_at`, deletes the record, clears the cookie, returns the redirect target.
- On any failure: deletes any found record, clears the cookie, returns the home path.

**Example — successful redirect target:**

```json
{
  "redirect_to": "/reports/8813"
}
```

**Example — expired intent:**

```json
{
  "redirect_to": "/"
}
```

---

## Long-Tail Tasks

### Validation

- Confirm the same-origin definition against the app's mount layout (single origin vs. multiple app mounts sharing one host) — see Open Question 3.
- Decide whether a permission-denied destination pre-checks to home or defers to the app's own authz — see Open Question 4.

### SSO seam

- Agree the `state` payload shape with the SSO-owning team: `state` carries only the intent id (composed with any existing CSRF nonce), never the path.

### Observability

- Emit a counter on capture, consume-success, and each consume-failure reason, so the ~40-tickets/month baseline can be measured down after launch.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep link → destination

**As an anonymous visitor** who opened a shared report URL, I want to land on that exact report after I sign in with my email and password, so that the link a colleague sent me actually takes me where it pointed.

**As an anonymous visitor** arriving from an invite email or a bookmarked dashboard, I want the page I asked for to survive the login step, so that I do not have to re-navigate from the home screen.

---

## SSO parity

**As an anonymous visitor** whose company uses SSO, I want the deep-link destination honored after the provider round trip, so that single sign-on users get the same experience as password users.

---

## Silent, safe fallback

**As a visitor** whose login took longer than the intent's lifetime, I want to land on the home screen without an error, so that an expired link never looks like a broken app.

**As a visitor** whose stored destination is no longer valid, I want to be quietly sent home rather than shown a failure, so that the login never dead-ends.

---

## Safety

**As a security-conscious user**, I want the intended path kept server-side and never exposed in a URL or a readable cookie, so that a shared or logged login URL cannot leak or be tampered into an open redirect.

**As the auth service owner**, I want each browser to hold at most one open intent, evicting the oldest, so that the session cache cannot be flooded with abandoned intents.


