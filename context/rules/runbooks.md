# SMOKE-RUNBOOK FORMAT

**Files:** `plans/*/runbooks/**`

**For reviewers / agents:** cite the stable `RULE-RUNBOOK-*` id when flagging a violation. The
lintable subset (docs + assert presence) is enforced by the `runbook-format` ACDSL verifier;
everything else is `[review]`.

**Scope:** smoke-test runbook collections persisted under `plans/<feature>/runbooks/`
(ACTION-REVIEW-VERIFY-004). The tool is the repo's discovered smoke-test tool (RULE-PLAN-060);
tool-specific mechanics below are written for Bruno — in a repo with a different tool, apply
the same MUSTs through that tool's equivalent mechanism.

---

**RULE-RUNBOOK-001** `[review]` — A runbook is a complete collection started from the repo's
template, never from scratch.

* For Bruno: `bruno.json`, `collection.bru`, `environments/`, at least one scenario folder with
  `folder.bru` and numbered request files, and a `README.md`.
* The repo's template location and creation target are declared by a repo fact — copy the
  template; do not hand-assemble the skeleton.

**RULE-RUNBOOK-002** `[review]` — The distribution artifact is a zip of the collection folder,
produced by a repo make target, generated on demand and never committed.

* Done means the consuming tool accepts it: unzip and open in the tool
  (ACTION-REVIEW-VERIFY-006).

**RULE-RUNBOOK-003** `[review]` — The collection is fully self-contained: the collection-level
docs block is the operator manual.

* It states: how to import, which environment to select, every secret var to fill and where its
  value comes from, account/data prerequisites, the run order, and the complete list of manual
  steps. A person holding only the zip can run the collection.

**RULE-RUNBOOK-004** `[review]` — Every request file carries a non-empty docs block: why the
call exists, what it proves, the expected outcome, and its manual step if it has one.

* Enforced (presence, non-empty) by the `runbook-format` verifier; content quality is review.

**RULE-RUNBOOK-005** `[review]` — Recurring values are predefined, referenced, never inlined.

* Shared values (base URL, app id/secret, forwarded-for header, standard test accounts) come
  from the template's environment file and collection-level headers; requests reference vars.
* Secrets exist only via the tool's secret mechanism (`vars:secret`) — never as plain values in
  any committed file.

**RULE-RUNBOOK-006** `[review]` — Programmatic-first: the collection runs headless via the
tool's CLI; every request asserts its outcome; chained values move via post-response scripts.

* A manual step (e.g. copying a token from a mailbox) is the exception, allowed only where
  automating it is disproportionate — a judgement call, made per case.
* Every manual step is declared twice: in the request's docs block and in the operator
  manual's manual-steps list. An undeclared manual step is a violation.
* The preferred fix for a recurring manual step is a test affordance in the product — record it
  as a tracked open item (ACTION-REVIEW-DOCS-004).

**RULE-RUNBOOK-007** `[review]` — The main smoke folder is a re-runnable round trip: the
account/data end where they started. A scenario that dirties the environment lives in its own
folder with its setup documented.

**RULE-RUNBOOK-008** `[review]` — Values interpolated into GET query strings are
percent-encoded via a pre-request script; the tool interpolates vars raw and the server decodes
literal `+` as a space.
