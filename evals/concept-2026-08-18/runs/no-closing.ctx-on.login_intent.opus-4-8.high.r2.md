## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** Claude (unattended concept run)
> **Date:** 2026-08-18

---

## Goals

- Capture a visitor's intended destination before they authenticate and send them there afterward, instead of dumping them on the generic home screen.
- Cut the ~40 support tickets/month about "the link did not work".
- Work identically for password login and SSO, with no change to the login form.
- Reject unsafe redirect targets (off-origin, oversized, stale) silently — no error, no open redirect.
- Add no new persistent store: reuse the existing session cache.

---

## User Flows

### Deep link → login → redirect (password)

**Goals:**
- An anonymous visitor opening `/reports/8813` lands back on `/reports/8813` after signing in with email + password.

**Options:**

**MVP**
- Store the intended path server-side against a short-lived intent id when `/login` renders. (~2d)
- Carry the intent id in a signed, http-only cookie — never the query string. (~1d)
- On session issue, consume the intent, validate the stored path, and redirect there. (~1–2d)
- Validate on store and again on consume: same-origin path only, ≤2 KB, no absolute or protocol-relative URLs. (~1d)

**Backlog**
- Remember destination across devices (explicitly out of scope for now).

**Challenges:**
- Open-redirect risk if a crafted path escapes the same-origin check.
- The intended path must never leak into logs or referrers via the query string.

**Approach:**
- Validate at both boundaries (store and consume); store only a relative path, reconstruct the absolute URL server-side at redirect time.
- Keep the id — not the path — in the cookie, and the path server-side in the session cache.

---

### SSO round trip

**Goals:**
- The same capture-and-redirect behavior when the user authenticates through the external SSO provider.

**Options:**

**MVP**
- Pass only the intent id through the provider's `state` parameter; the path never leaves our backend. (~2d)
- On SSO callback, read the intent id from `state`, consume it exactly as the password flow does. (~1d)

**Backlog**
- Support multiple concurrent SSO providers (only the current one is in scope).

**Challenges:**
- The signed intent cookie may not survive the cross-site provider redirect (SameSite behavior).
- The provider integration is owned by another team; we only pass `state` through.

**Approach:**
- Treat `state` as the authoritative carrier of the intent id across the round trip; do not depend on the cookie surviving the provider hop.
- Cross-check the `state` intent id against the cookie when the cookie is present; fall back to `state` alone when it is not.

---

### Expired or invalid intent

**Goals:**
- A stale or malformed intent degrades gracefully to the home screen.

**Options:**

**MVP**
- After the 15-minute TTL, or when the stored path fails validation on consume, land the user on the home screen with no error and delete the intent record. (~1d)

**Backlog**
- Surface a subtle "your link expired" hint (deferred; risks noise).

**Challenges:**
- A visibly failed redirect erodes trust more than a silent fallback.

**Approach:**
- Fail closed and quiet: no error banner, delete the record, log at debug for support triage only.

---

## Decisions / Open Questions

**Decisions:**
- Intent id lives in a signed http-only cookie; the path lives server-side in the session cache — the path never travels in a URL.
- SSO carries only the intent id in `state`, never the path.
- No new persistent store; the existing session cache backs intent records.
- One open intent per browser; storing a new intent evicts the oldest.
- Invalid/expired intents fall back silently to the home screen and are deleted.
- Validation runs at both store time and consume time (defense in depth), since the cache could be tampered with or the ruleset could change between the two calls.

**Open Questions:**
1. **Home screen path.** Assumed the post-login fallback is `/` (or the app's configured default landing route). Confirm the exact default.
2. **Cookie name & signing key.** Assumed a new cookie `li` signed with the existing session-cookie secret. Confirm whether a dedicated key is required.
3. **SameSite for the intent cookie.** Assumed `SameSite=Lax` so the top-level redirect back from a deep link carries it, while the SSO flow leans on `state`. Confirm this is acceptable for the SSO provider's redirect behavior.
4. **Session cache key namespace & eviction.** Assumed intent records key on `intent:{id}` with a 15-minute TTL, and "one open intent per browser" is enforced by keying the *current* intent id off the cookie and overwriting. Confirm the cache supports per-key TTL and that this eviction model is sufficient (vs. a per-browser index).
5. **Returning user with a live session.** Assumed a returning user who hits `/reports/8813` with a valid session cookie is served directly and never touches the intent flow. Confirm no capture is needed when already authenticated.
6. **Non-GET deep links.** Assumed only GET navigations are captured; a deep link is always a GET. Confirm no POST/other-method targets need preserving.
7. **Login page re-render.** Assumed each `/login` render creates a fresh intent (overwriting any prior one for that browser), so a refresh of the login page does not orphan records. Confirm this is desired over reusing an existing live intent.


## file: plans/login_intent/concept/deep_link_redirect.md

# Deep Link → Login → Redirect (Password)

Detailed design for the password-login flow: capture the intended path when `/login` renders, honor it when the session is issued.

---

## Flows

### Capture and redirect (happy path)

1. Anonymous visitor opens `/reports/8813`.
2. The web app detects no session and redirects the browser to `/login`.
3. Backend (on `/login` render — **create intent**)
   1. Read the pre-auth destination (the original path the visitor was sent from).
   2. Validate the path: same-origin relative path, ≤2 KB, not absolute, not protocol-relative. Reject → skip capture, render login normally.
   3. Generate a random intent id.
   4. Evict any existing open intent for this browser (see Limits), then store `{id, path, created_at, consumed_at=null}` in the session cache under `intent:{id}` with a 15-minute TTL.
   5. Set a signed, http-only cookie carrying only the intent id.
4. Visitor submits email + password on the unchanged login form.
5. Backend (on session issue — **consume intent**)
   1. Read the intent id from the signed cookie; verify the signature. Missing/invalid signature → fall back to home.
   2. Load `intent:{id}` from the cache. Missing/expired → fall back to home, clear the cookie.
   3. Re-validate the stored path against the same rules. Fail → delete the record, clear the cookie, fall back to home.
   4. Mark `consumed_at`, delete the record (single-use), clear the intent cookie.
   5. Issue the session and return a redirect to the validated path.
6. Visitor lands on `/reports/8813`.

### Expired or invalid intent

1. Visitor reaches the login form more than 15 minutes after capture, or the stored path fails re-validation.
2. Backend (consume intent)
   1. Cache lookup misses (TTL expired) or validation fails.
   2. Delete any surviving record, clear the intent cookie.
   3. Issue the session and redirect to the home screen. No error surfaced.
3. Visitor lands on the home screen.

### Returning user with a live session

1. Visitor with a valid session cookie opens `/reports/8813`.
2. Backend serves the report directly; the intent flow is never entered.

---

## Security Considerations

- **Open redirect.** Only same-origin relative paths are stored, validated at both store and consume time. Absolute URLs (`https://evil.example/...`) and protocol-relative URLs (`//evil.example/...`) are rejected. The redirect URL is reconstructed server-side from the stored relative path, never echoed from client input at consume time.
- **Path leakage.** The intended path lives only server-side; the client holds an opaque intent id in an http-only cookie. Nothing sensitive rides the query string, so the path cannot leak via referrer headers, browser history, or access logs.
- **Cookie tampering / forgery.** The intent cookie is signed; a bad signature is treated as no intent. Forging a valid id still only yields another user's *relative path*, which is re-validated and is same-origin by construction — no cross-account data exposure.
- **Intent fixation / replay.** Intents are single-use: consumed records are deleted, so a captured id cannot be replayed after login.
- **Cache exhaustion.** One open intent per browser plus a 15-minute TTL bounds growth; oldest-eviction prevents a single browser from accumulating records.

---

## Limits

- **TTL:** 15 minutes (a login attempt should complete well within this; long enough to survive a password-reset detour, short enough to bound cache use).
- **Max path length:** 2 KB (accommodates long report/query paths; caps cache entry size and blocks oversized-payload abuse).
- **Path scope:** same-origin relative paths only (prevents open redirect).
- **Open intents per browser:** 1 (a browser has at most one pending destination; storing a new intent evicts the oldest).
- **Single-use:** an intent is deleted on consume (prevents replay).

---

## Models

### Intent

**Public:**
- id: opaque random identifier carried in the signed cookie / SSO `state` (string)
- path: same-origin relative destination path (string)

**Internal / Not Exported:**
- created_at: capture timestamp, basis for the 15-minute TTL (timestamp)
- consumed_at: set when the intent is honored; the record is deleted immediately after, so this is transient (timestamp, nullable)

**Unique Index:**
- id (cache key `intent:{id}`)

**Storage:** the existing session cache — no new persistent store. Records expire by the cache's native per-key TTL.

---

## APIs

Both calls are internal backend steps, not new public endpoints. They are described as the two logical operations the request handlers perform.

### create_intent (during `GET /login` render)

Captures the pre-auth destination and hands the browser a signed intent id.

**Notes:**
- No-op (renders login without an intent cookie) when there is no destination or validation fails.
- Evicts the browser's prior open intent before storing the new one.
- The login form markup is unchanged; this operates in the render handler only.

**Request fields:**
- destination_path: the path the visitor was redirected from (string, server-derived)

**Request headers:**
- Cookie (existing intent cookie, if any — used for eviction)

**Response fields:**
- Set-Cookie: signed, http-only intent cookie carrying the intent id

**Rate Limits:**
Inherits the login-page render rate limit; no separate limit.

**Example (200):**

Request:
```json
{ "destination_path": "/reports/8813" }
```

Response:
```json
{ "intent_id": "aQ8kZ2r7Xn", "set_cookie": "li=<signed>; HttpOnly; SameSite=Lax; Path=/" }
```

### consume_intent (during session issue)

Validates and returns the redirect target, then deletes the intent.

**Notes:**
- Called on both password-login success and SSO callback.
- Returns the home path (not an error) on any miss, expiry, or validation failure.
- Deletes the record and clears the cookie on every outcome.

**Request fields:**
- intent_id: from the signed cookie (password flow) or the SSO `state` (SSO flow) (string)

**Request headers:**
- Cookie (signed intent cookie)

**Response fields:**
- redirect_path: the validated destination, or the home path on fallback (string)
- Set-Cookie: cleared intent cookie

**Rate Limits:**
Inherits the login / SSO-callback rate limit; no separate limit.

**Example (200 — honored):**

Request:
```json
{ "intent_id": "aQ8kZ2r7Xn" }
```

Response:
```json
{ "redirect_path": "/reports/8813" }
```

**Example (200 — expired, silent fallback):**

Request:
```json
{ "intent_id": "aQ8kZ2r7Xn" }
```

Response:
```json
{ "redirect_path": "/" }
```

---

## Worker Tasks

- None. Expiry is handled by the session cache's native TTL; no sweeper job is required.

---

## Infrastructure

- No new store or service. Reuses the existing session cache and the existing cookie-signing secret (pending Open Question 2).
- One new signed http-only cookie (`SameSite=Lax`, pending Open Question 3).

---

## Long-Tail Tasks

### Validation

- Nail down the exact same-origin relative-path predicate (leading `/`, reject `//`, reject scheme, normalize `.`/`..` segments, reject after normalization if it escapes root).

### SSO handoff (see the SSO flow in concept.md)

- Confirm with the SSO-owning team that `state` is free to carry our intent id and is returned verbatim.
- Decide cookie-vs-`state` precedence when both are present on the SSO callback.

### Observability

- Debug-level log on silent fallback (expiry/invalid) so support can correlate residual "link did not work" reports without exposing paths at info level.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep-link return (password)

**As an anonymous visitor** who opened a shared report link, I want to land on that report after I sign in with my email and password, so that I do not have to hunt for it again from the home screen.

**As an anonymous visitor**, I want the link I followed to keep working even if I take a couple of minutes to log in, so that a short delay does not lose my destination.

---

## Deep-link return (SSO)

**As an anonymous visitor** whose company uses SSO, I want to land on the report I opened after the SSO round trip, so that single sign-on does not cost me my destination.

---

## Safe fallback

**As an anonymous visitor** whose link has expired, I want to land quietly on the home screen, so that I am not blocked by a confusing error.

**As a security-conscious user**, I want a tampered or off-site redirect target to be ignored, so that a malicious link cannot bounce me somewhere unsafe after I authenticate.

---

## Returning user

**As a returning user** with a live session, I want a deep link to open directly, so that I am never sent through the login detour unnecessarily.

---

## Support & operations

**As a support agent**, I want "the link did not work" reports to drop off, so that I spend less time on avoidable tickets.

**As the auth service owner**, I want the intended path kept server-side and out of URLs and logs, so that destinations are not leaked through referrers or query strings.


