## file: plans/login_intent/concept/concept.md

# Concept: Login Intent Capture

> **Status:** Draft
> **Author:** concept skill (unattended)
> **Date:** 2026-08-18

---

## Goals

- Capture a visitor's intended destination before they authenticate and return them to it after sign-in, for both password login and SSO.
- Cut the ~40 support tickets/month about shared or bookmarked deep links "not working".
- Keep the intended path server-side and out of URLs; the browser only ever holds an opaque intent id in a signed cookie.
- Add no new persistent store — reuse the existing session cache.
- Leave the login form and the externally-owned SSO integration untouched (state pass-through only).

---

## User Flows

### Flow 1 — Deep link → password login → redirect

**Goals:**
- A visitor who opens a deep link while logged out lands on that exact path after signing in with email + password.

**Options:**

**MVP**
- Store the intended path server-side against a short-lived intent id when the login page renders (`~2d`).
- Issue a signed, HTTP-only intent-id cookie; never place the id or path in the query string (`~1d`).
- On session issue, consume the intent and redirect to the stored path (`~1d`).
- Validate paths on capture and again on consume: same-origin, path-only, ≤2 KB (`~1d`).

**Backlog**
- Surface a "you'll be returned to X after login" hint on the login page (`~1d`).

**Challenges:**
- The intent must be readable at session-issue time regardless of which handler issues the session.
- A stale or tampered cookie must never redirect a user to an attacker-chosen location.

**Approach:**
- Bind the intent record to the intent id carried in the signed cookie; the signature guarantees the id is one we issued.
- Re-validate the stored path at consume time; anything that fails validation falls through to the home screen.

**Block total (MVP): ~5–6d.** Detailed design: `intent_lifecycle.md`.

---

### Flow 2 — SSO round trip

**Goals:**
- The same return-to-destination behavior when the visitor authenticates through the external SSO provider.

**Options:**

**MVP**
- Carry the intent id (never the path) through the provider's `state` parameter so it survives the redirect out and back (`~2d`).
- On the SSO callback that issues the session, consume the intent exactly as the password flow does (`~1d`).

**Backlog**
- Cross-check that the `state` intent id matches the intent-id cookie, when the cookie survives the round trip (`~1d`).

**Challenges:**
- The SSO integration is owned by another team; we can only pass `state` through it, not change its contract.
- The intent-id cookie may or may not accompany the provider callback depending on cross-site cookie behavior.

**Approach:**
- Treat the `state` intent id as the authoritative carrier; do not rely on the cookie surviving the provider hop.
- Keep `state` limited to the opaque id so no path data leaves our origin.

**Block total (MVP): ~3d.**

---

### Flow 3 — Expired or invalid intent

**Goals:**
- A visitor whose intent has expired or fails validation lands on the home screen silently, with the record cleaned up.

**Options:**

**MVP**
- On consume, if the intent is missing, past its 15-minute TTL, or its stored path fails re-validation, redirect to the home screen with no error shown (`~1d`).
- Delete the intent record on consume regardless of outcome (`~0.5d`).

**Backlog**
- Emit a metric distinguishing expired vs. invalid vs. missing intents for support triage (`~1d`).

**Challenges:**
- Expiry must not surface as an error the user has to dismiss.
- A consumed or expired record must not be replayable.

**Approach:**
- Model "no valid intent" and "expired intent" as the same user-visible outcome: home screen, no message.
- Rely on the session-cache TTL for expiry and delete-on-consume for single use.

**Block total (MVP): ~1.5d.**

---

## Decisions / Open Questions

**Decisions:**
- Intended path lives server-side in the existing session cache; the browser holds only a signed intent-id cookie.
- Path validation (same-origin, path-only, ≤2 KB) runs both at capture and at consume; absolute and protocol-relative URLs are rejected.
- TTL is 15 minutes, enforced by the session-cache entry.
- One open intent per browser; creating a new intent evicts the oldest.
- SSO `state` carries only the intent id, never the path.
- Failure modes (expired, invalid, missing) all resolve to the home screen with no error.

**Open Questions:**
1. What is the canonical "home screen" path to redirect to on fallback (`/` vs. a per-user default dashboard)? Assumed `/` for the MVP.
2. Cookie attributes for the intent-id cookie under the SSO round trip: is `SameSite=Lax` sufficient for the provider callback, or is `SameSite=None; Secure` required? Assumed `Lax`, with `state` as the authoritative carrier so the cookie is not load-bearing across the hop.
3. Should a returning user who already has a valid session and opens a deep link ever hit the login page at all, or bypass intent capture entirely? Assumed bypass — an authenticated request to `/reports/8813` serves the page directly and never creates an intent.
4. Is the cookie signing key the existing session-signing key, or a separate key? Assumed reuse of the existing session-signing key.
5. What is the maximum acceptable size of the SSO `state` value the provider will echo back? Assumed an opaque id ≤128 bytes fits comfortably; needs confirmation from the SSO-owning team.
6. On concurrent tabs each creating an intent, the "oldest evicted" rule means an earlier tab's deep link is dropped. Is that acceptable, or should the limit be per-tab rather than per-browser? Assumed per-browser with oldest-evicted for the MVP.


## file: plans/login_intent/concept/intent_lifecycle.md

# Intent Lifecycle (Flow 1 — Password Login)

Covers the create-on-render / consume-on-session-issue lifecycle for the deep-link → password-login → redirect flow. The SSO flow reuses the same model and consume call; only the id carrier differs (`state` parameter instead of the redirect that lands the visitor on `/login`).

---

## Flows

### Deep link → login → redirect (happy path)

1. Anonymous visitor opens `/reports/8813`.
2. The web app sees no session and redirects to `/login`, preserving the original path.
3. Backend (login-page render)
   1. Validate the original path: same-origin, path-only, ≤2 KB. Reject absolute (`https://…`) and protocol-relative (`//host/…`) forms.
   2. Create an intent record in the session cache: fresh `id`, `path`, `created_at`, `consumed_at = null`, TTL 15 min.
   3. Enforce one-open-intent-per-browser: if a valid intent-id cookie is already present, delete that record before storing the new one (oldest evicted).
   4. Set a signed, HTTP-only intent-id cookie holding only the `id`.
4. Visitor submits email + password on the unchanged login form.
5. Backend (session issue)
   1. Authenticate credentials and issue the session as today.
   2. Read the intent id from the signed cookie; verify the signature.
   3. Load the intent record; if missing or past TTL, fall through to the home screen.
   4. Re-validate the stored `path` (same rules as capture). On failure, fall through to the home screen.
   5. Set `consumed_at`, then delete the intent record and clear the intent-id cookie.
   6. Redirect to the stored `path` (`/reports/8813`).

### Expired or invalid intent (fallback)

1. Visitor signs in after the 15-minute TTL, or the stored path fails re-validation.
2. Backend
   1. Signature verify and lookup as above; record is absent/expired, or re-validation fails.
   2. Delete any lingering record and clear the intent-id cookie.
   3. Redirect to the home screen with no error surfaced.

---

## Security Considerations

- **Open-redirect via tampered destination** — the path never travels in a URL or cookie; only an opaque id does. The id is signature-verified, and the stored path is re-validated at consume time, so a forged cookie cannot steer the redirect off-origin.
- **Intent-id forgery** — the cookie is signed with the existing session-signing key; an unsigned or bad-signature id is treated as no intent.
- **Replay** — delete-on-consume plus `consumed_at` make an intent single-use; the 15-minute TTL bounds the replay window even if deletion is missed.
- **Cross-user leakage** — the intent is bound to the browser's cookie, not to a user identity (the visitor is anonymous at capture), and is consumed within the same browser session.
- **Path length / DoS** — the 2 KB cap bounds stored size; the per-browser single-intent limit bounds cache footprint per client.
- **Cookie theft** — HTTP-only and Secure attributes keep the id out of JavaScript and off plaintext transport; the id alone reveals no destination.

---

## Limits

- **TTL:** 15 minutes (short-lived; a login attempt is expected within one sitting).
- **Max path length:** 2 KB (generous for real paths, bounds cache use).
- **Open intents per browser:** 1 (oldest evicted on new capture — avoids unbounded growth from repeated deep-link visits).
- **Path scheme:** same-origin path-only; absolute and protocol-relative URLs rejected (open-redirect prevention).
- **Cookie:** signed, HTTP-only, Secure; holds only the intent id.

---

## Models

### Intent

**Public:**
- id: opaque identifier carried by the cookie and, for SSO, the `state` parameter (string)
- path: the same-origin destination path to return to (string)

**Internal / Not Exported:**
- created_at: capture timestamp; TTL is measured from here (timestamp)
- consumed_at: set when the intent is redeemed; null while open (timestamp, nullable)

**Unique Index:**
- id

Stored as a session-cache entry keyed by `id` with a 15-minute expiry; no new persistent store.

---

## APIs

Both are internal endpoints called by existing handlers, not public API surface. They may be implemented as inline handler steps rather than separate routes; described here as calls for clarity.

### POST /internal/intents

Create an intent when the login page renders for a redirected visitor.

**Notes:**
- Called only when an anonymous request is redirected to `/login` with an original path.
- Validates the path before storing; an invalid path yields no intent and no cookie (the login page still renders).
- Evicts the caller's existing open intent, if any, before creating the new one.

**Request fields:**
- path: the original same-origin destination path (string)

**Response fields:**
- id: the created intent id, also set as a signed cookie (string)

**Rate Limits:**
Bounded implicitly by the one-open-intent-per-browser rule; per-IP create rate reuses the login-page's existing limiter.

**Example (201):**

Request:
```json
{ "path": "/reports/8813" }
```

Response:
```json
{ "id": "itn_9f3c2ae7b104" }
```

### POST /internal/intents/consume

Consume the intent at session issue and resolve the redirect target.

**Notes:**
- Reads the intent id from the signed cookie (password flow) or the SSO `state` parameter (SSO flow).
- Re-validates the stored path; deletes the record and clears the cookie regardless of outcome.
- Returns the home path when the intent is missing, expired, or fails re-validation — never an error status the UI must handle.

**Request headers:**
- Cookie: the signed intent-id cookie (password flow)

**Request fields:**
- id: intent id, when supplied via SSO `state` instead of the cookie (string, optional)

**Response fields:**
- redirect_path: the validated destination, or the home path on any fallback (string)

**Rate Limits:**
Bounded by session-issue frequency; no separate limiter.

**Example (200, happy path):**

Request:
```json
{}
```

Response:
```json
{ "redirect_path": "/reports/8813" }
```

**Example (200, expired/invalid fallback):**

Request:
```json
{}
```

Response:
```json
{ "redirect_path": "/" }
```

---

## Worker Tasks

- None required. Expiry is handled by the session-cache TTL; single-use is handled by delete-on-consume. No sweeper job needed.

---

## Infrastructure

- No new store or resource. Reuses the existing session cache and the existing cookie-signing key.
- SSO integration unchanged: the SSO-owning team's contract is used as-is, passing the intent id through the `state` parameter only.

---

## Long-Tail Tasks

### Validation

- Centralize the same-origin path validator so capture and consume share one implementation (avoids drift between the two checkpoints).
- Confirm the rejection set covers protocol-relative (`//host`), backslash (`/\host`), and encoded-scheme edge cases.

### SSO carrier

- Confirm with the SSO-owning team the maximum `state` length echoed back, and that `state` is returned verbatim.
- Decide whether to cross-check the `state` id against a surviving intent-id cookie when present (Backlog item in Flow 2).

### Fallback path

- Resolve the canonical home path (Open Question 1) before implementation — `/` vs. a per-user default dashboard.


## file: plans/login_intent/concept/user_stories.md

# User Stories: Login Intent Capture

---

## Deep-link return (password login)

**As an anonymous visitor** who opened a shared report link, I want to sign in with my email and password and land on that report, so that I don't have to hunt for it after logging in.

**As an anonymous visitor**, I want the destination remembered even though it never appears in the URL bar, so that the link I was sent can't be leaked or tampered with through the address I'm redirected to.

---

## Deep-link return (SSO)

**As an anonymous visitor** whose company uses single sign-on, I want to reach my intended page after the SSO round trip, so that the return-to-destination works the same whether or not I use a password.

**As a security-conscious org**, I want only an opaque id — not the destination path — to travel through the SSO provider, so that no internal URLs leak to a third party.

---

## Graceful fallback

**As an anonymous visitor** whose link sat unused past its expiry, I want to land on the home screen without an error, so that a stale link is a mild inconvenience rather than a dead end.

**As an anonymous visitor**, I want a malformed or off-site destination to be ignored, so that I'm never redirected somewhere unexpected or unsafe.

---

## Already signed in

**As a returning user** with an active session, I want a deep link to open directly, so that I'm not sent through the login page for a destination I could already see.

---

## Support and operations

**As a support agent**, I want deep links to reliably return users to their destination, so that the recurring "the link did not work" tickets go away.


