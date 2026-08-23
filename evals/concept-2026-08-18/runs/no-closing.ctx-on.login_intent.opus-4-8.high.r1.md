## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** Claude
> **Date:** 2026-08-18

---

## Goals

- Capture the destination a visitor intended to reach before they were sent to `/login`, and honor it after authentication.
- Support both password login and SSO round trips with a single intent mechanism.
- Reduce the ~40 support tickets/month about "the link did not work".
- Store the intent server-side, never expose the destination path in the URL, and reuse the existing session cache (no new persistent store).
- Fail silently: an expired, missing, or invalid intent lands the user on the home screen with no error.

---

## User Flows

### Deep link → password login → redirect

**Goals:**
- An anonymous visitor who opens a same-origin deep link (e.g. `/reports/8813`) lands on that exact path after signing in with email + password.

**Options:**

**MVP**
- Capture the intended path server-side when the login page renders, keyed by a short-lived intent id. (~2d)
- Carry the intent id in a signed, HttpOnly cookie — never in the query string. (~1d)
- On session issue, consume the intent, validate the stored path, and redirect there. (~1d)

**Backlog**
- Surface the intended destination on the login page ("Sign in to view report #8813"). (~1d)

**Challenges:**
- The login form must not change — capture and consume happen around it, not inside it.
- Path validation must reject open-redirect vectors (absolute URLs, protocol-relative URLs, non-same-origin paths).

**Approach:**
- Create intent on login-page render; consume on session issue. The form POST is untouched; the intent id rides the cookie the server already sets on that render.
- Validate on both write (create) and read (consume) so a tampered cache entry still fails closed.

### SSO round trip

**Goals:**
- The same deep-link redirect works when the visitor authenticates through the external SSO provider.

**Options:**

**MVP**
- Pass the intent id (only the id, never the path) through the SSO `state` parameter so it survives the provider redirect. (~2d)
- On SSO callback / session issue, read the intent id from `state`, consume the intent, and redirect. (~1d)

**Backlog**
- Reconcile the `state`-carried id against the signed cookie when both are present, preferring the cookie. (~1d)

**Challenges:**
- The SSO provider integration is owned by another team; we only pass `state` through it.
- The signed cookie may not survive the cross-origin provider hop, so `state` is the authoritative carrier for SSO.

**Approach:**
- Treat `state` as the intent-id carrier for the SSO leg; keep the path server-side the entire time. The other team's contract is unchanged — they echo `state` back verbatim.

### Expired or invalid intent

**Goals:**
- After the 15-minute TTL, or when the stored path fails validation, the user lands on the home screen with no error and the intent record is deleted.

**Options:**

**MVP**
- On consume: if the intent is missing, expired, or its path fails validation, delete the record and redirect to home. (~1d)

**Challenges:**
- Must be indistinguishable from a normal home-screen login — no error banner, no flash.

**Approach:**
- Consume is idempotent and best-effort: any failure collapses to the home redirect and a delete.

---

## Decisions / Open Questions

**Decisions:**
- Intent lives in the existing session cache, not a new store (constraint from intake).
- Intent id travels in a signed HttpOnly cookie for password login and in the SSO `state` parameter for SSO; the path is always server-side.
- Validation runs on both create and consume; same-origin absolute paths only.
- One open intent per browser; creating a new one evicts the oldest.
- Failures are silent: home redirect + record delete.

**Open Questions:**
1. Cookie name and attributes — assumed `li` (login intent), `Secure; HttpOnly; SameSite=Lax` (Lax so the SSO redirect back to our origin still presents it). Confirm `SameSite` value against the SSO provider's redirect method (302 GET vs form POST).
2. Signing key source — assumed the existing session-cookie signing key is reused. Confirm whether a separate key is wanted.
3. "One open intent per browser" scoping — assumed keyed by the intent cookie; a browser with no cookie yet always gets a fresh intent. Confirm whether eviction is per-browser or global-oldest under cache pressure.
4. Path allowlist granularity — assumed any same-origin path starting with `/` (single slash, not `//`) is valid. Confirm whether specific route prefixes (e.g. `/reports`, `/dashboard`) should be allowlisted instead of accepting all same-origin paths.
5. Returning users with a live session who hit a deep link — assumed they bypass `/login` entirely and no intent is created. Confirm no intent bookkeeping is wanted for the already-authenticated case.
6. Intent creation trigger — assumed intent is created only when the login page is reached via redirect from a protected path, not on a direct visit to `/login`. Confirm direct `/login` visits should carry no intent (redirect to home post-auth).


## file: plans/login_intent/concept/deep_link_redirect.md

# Deep Link Redirect (Password Login)

The intent capture-and-consume mechanism for flow 1: an anonymous visitor opens a same-origin deep link, is sent to `/login`, signs in with email + password, and lands on the original path.

---

## Flows

### Create intent (login-page render)

1. Visitor opens `/reports/8813` without a session.
2. The web app redirects to `/login`, preserving the originally-requested path server-side.
3. Backend
   1. Validate the requested path against the validation rules below. If it fails, skip intent creation and render `/login` with no intent cookie.
   2. Generate a random intent id.
   3. If the request already presents a valid intent cookie, delete that prior intent record (one open intent per browser — oldest evicted).
   4. Write the intent record to the session cache keyed by intent id, TTL 15 minutes.
   5. Set a signed, HttpOnly, Secure cookie carrying the intent id (`SameSite=Lax`).
4. Render `/login`. The login form itself is unchanged.

### Consume intent (session issue, password)

1. Visitor submits email + password; auth service issues a session.
2. Backend
   1. Read the intent id from the signed cookie; verify the signature. On failure, redirect home.
   2. Look up the intent record in the session cache. If missing or expired, delete any record and redirect home.
   3. Re-validate the stored path against the validation rules. If it fails, delete the record and redirect home.
   4. Mark the record consumed (`consumed_at`) and delete it from the cache.
   5. Clear the intent cookie.
3. Redirect the now-authenticated user to the stored path (`/reports/8813`).

### Expired or invalid intent

1. Visitor completes login more than 15 minutes after intent creation, or the stored path no longer validates.
2. Backend
   1. Cache lookup misses (TTL expiry) or re-validation fails.
   2. Delete any lingering record and clear the intent cookie.
3. Redirect to the home screen with no error shown.

---

## Security Considerations

- **Open redirect** — the stored path is validated on both create and consume; only same-origin paths beginning with a single `/` are accepted. Absolute URLs (`https://evil.example`), protocol-relative URLs (`//evil.example`), and backslash variants (`/\evil.example`) are rejected. Re-validation on consume means a tampered cache entry still fails closed.
- **Path disclosure** — the destination path never appears in a URL or query string; only the opaque intent id is client-visible, and it rides a signed cookie.
- **Intent-id forgery** — the cookie is signed; an unverifiable signature is treated as no intent (home redirect).
- **Cookie theft / fixation** — cookie is HttpOnly and Secure; the record is single-use (deleted on consume) and short-lived (15-minute TTL), bounding the replay window.
- **Cache exhaustion** — one open intent per browser with oldest-evicted, plus the 2 KB path cap and TTL, bound per-browser and aggregate cache footprint.

---

## Limits

- **TTL**: 15 minutes (short enough to bound replay, long enough to complete a login incl. password reset detour).
- **Max path length**: 2 KB (rejects oversized paths before they reach the cache).
- **Open intents per browser**: 1 (creating a new intent evicts the browser's prior record).
- **Path scheme**: same-origin only — must begin with a single `/`, must not begin with `//` or `/\`, must not contain a scheme (`:` before the first `/`).

---

## Models

### Intent

**Public:**
- id: opaque random identifier carried in the signed cookie / SSO `state` (string)

**Internal / Not Exported:**
- path: the same-origin destination path to redirect to after auth (string, ≤ 2 KB)
- created_at: creation timestamp; drives the 15-minute TTL (timestamp)
- consumed_at: set when the intent is consumed on session issue; nil while open (timestamp, nullable)

**Unique Index:**
- id (cache key)

**Storage:** existing session cache, no new persistent store. Record expires via cache TTL at `created_at + 15m`.

---

## APIs

### POST /internal/intents

Create an intent when the login page renders after a redirect from a protected path. Internal call from the web app's login-page handler, not a public endpoint.

**Notes:**
- Called only when the login page is reached via redirect carrying an original path; a direct `/login` visit creates no intent.
- Validates the path before writing; an invalid path returns no intent id and sets no cookie.
- Evicts the caller's prior open intent (one per browser) before writing the new record.

**Request fields:**
- path: the originally-requested same-origin path (string)

**Request headers:**
- Cookie (optional prior intent cookie, for eviction)

**Response fields:**
- id: the new intent id (string)

**Response headers:**
- Set-Cookie: signed, HttpOnly, Secure intent cookie carrying `id`

**Rate Limits:**
Per-browser; bounded implicitly by one-open-intent eviction.

**Example (201):**

Request:
```json
{ "path": "/reports/8813" }
```

Response:
```json
{ "id": "aQ9f2kLpZ" }
```

### POST /internal/intents/consume

Consume the intent at session issue and resolve the post-login redirect target. Internal call from the session-issue path (password and SSO).

**Notes:**
- Idempotent and best-effort: a missing, expired, or invalid intent returns the home path rather than an error.
- Deletes the record and clears the cookie regardless of outcome.
- For password login the id comes from the signed cookie; for SSO it comes from the `state` parameter.

**Request fields:**
- id: the intent id from the signed cookie or SSO `state` (string)

**Request headers:**
- Cookie (signed intent cookie, password flow)

**Response fields:**
- path: the validated destination path, or the home path (`/`) on any failure (string)

**Response headers:**
- Set-Cookie: cleared intent cookie

**Rate Limits:**
Per-session-issue; one consume per login.

**Example (200) — hit:**

Request:
```json
{ "id": "aQ9f2kLpZ" }
```

Response:
```json
{ "path": "/reports/8813" }
```

**Example (200) — expired/invalid/miss:**

Request:
```json
{ "id": "aQ9f2kLpZ" }
```

Response:
```json
{ "path": "/" }
```

---

## Long-Tail Tasks

### Validation

- Centralize the same-origin path validator so create and consume share one implementation (reject absolute, protocol-relative, backslash, and scheme-bearing paths).

### Cookie handling

- Confirm `SameSite=Lax` survives the SSO provider's redirect-back method; escalate to the SSO-owning team if the cookie is dropped on the cross-origin hop.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep-link redirect (password login)

**As an anonymous visitor** who opens a shared same-origin deep link, I want to sign in with email and password and land on the exact page I requested, so that the shared link works as expected.

**As an anonymous visitor**, I want the page I was trying to reach to be remembered even though I never see it in the URL, so that my destination is not leaked or tampered with.

---

## SSO redirect

**As an anonymous visitor** who signs in through the SSO provider, I want my intended deep-link destination to survive the provider round trip, so that SSO users get the same redirect as password users.

---

## Graceful failure

**As a visitor** whose saved destination has expired (more than 15 minutes old), I want to land on the home screen without seeing an error, so that a stale link is not a dead end.

**As a visitor** whose saved destination is invalid (not a same-origin path), I want to be sent to the home screen instead of an unexpected or unsafe location, so that I am never redirected off-site.

---

## Returning user

**As a returning user** with an active session who opens a deep link, I want to reach that page directly without a login detour, so that I am not prompted to sign in again.

---

## Operations

**As a support agent**, I want deep links to resolve after login, so that I stop fielding "the link did not work" tickets.

**As a security reviewer**, I want the intended path stored server-side and validated on read and write, so that the redirect cannot be abused as an open-redirect vector.


