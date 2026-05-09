# Research: Required Author field on Report Feedback

## R1: Author character cap value

**Decision**: 80 characters.

**Rationale**: The existing `titleMaxChars` is 200; an Author display
name does not need anywhere near that. 80 fits "First Last (Team
Name)" comfortably and matches the conventional terminal/title row
width that triagers scan in a GitHub issue list. Smaller than 200
also tightens the inline-validation surface for a clearer user error
message ("Author must be 80 characters or fewer").

**Alternatives considered**:
- 64: too tight for "First Middle Last (Long Team Name)" patterns.
- 200 (mirror title): wasteful; an author isn't free-form prose.
- Unbounded: violates the existing payload-discipline pattern in the
  worker contract. Every other string field has an explicit cap.

## R2: Where the "Submitted by:" line is composed

**Decision**: Compose the line in `feedback-worker/src/body.ts`
(worker side), and mirror it in the legacy `maybePrefixBody` in
`render/assets/app-js/03.js` (the `window.open` GitHub fallback URL
builder).

**Rationale**: The worker is already the single canonical body
composer (it injects the `> Category:` quote and the
`_Submitted via my-gather Report Feedback (v...)._` trailer). Putting
the `Submitted by:` line there keeps the canonical owner intact
(Principle XIII). The legacy fallback bypasses the worker entirely
(it pre-fills GitHub via URL params), so its body builder
(`maybePrefixBody`) MUST emit the same line itself; this is documented
external degradation under Principle III, and the same observable line
format makes the two paths converge.

**Alternatives considered**:
- Compose on the frontend, send a pre-baked body to the worker:
  rejected because it would split the canonical composer across two
  places (worker still adds category + trailer) and break the existing
  pattern. Also makes worker-side validation fragile (would have to
  parse author back out of the body string).
- Compose only on the worker, leave fallback bare: rejected because
  the same input would produce two different observable issue bodies
  depending on whether the worker was reachable, breaking Principle
  XIII canonical observable behaviour.

## R3: Line position within the issue body

**Decision**: `Submitted by: <name>` is the FIRST line of the body,
placed before the existing `> Category: <cat>` quote (when present)
and before the user-typed body. A trailing blank line separates the
attribution block from the user content, mirroring the existing blank
line after `> Category:`.

**Rationale**: Triagers scan the top of the issue body first. Author
attribution is the most-relevant context for triage routing
("who do I follow up with?"), so it earns the top slot. The category
quote stays where it is, just one line lower.

**Alternatives considered**:
- Put it in the GitHub issue title: rejected — the title is
  user-controlled and short; mixing identity into it pollutes search
  ("show me all 'index missing' bugs" would not match
  "Mati: index missing"). Body line keeps title clean.
- Put it in the trailer next to the version footer: rejected —
  triagers need it BEFORE they read the body, not after.

## R4: Persistence storage choice

**Decision**: Browser `localStorage` under the single key
`mygather.feedback.lastAuthor`, plain text (the Author value as-is).

**Rationale**: `localStorage` is universally available in modern
browsers, persists across tab close, and is scoped per origin — but
the report is a single static `file://` document, so each
report-on-disk shares localStorage with itself across opens; an
operator who saves multiple reports under the same `file://` host gets
shared persistence, which is the desired behaviour for "don't retype
my name". `sessionStorage` would lose the value across tab closes.
IndexedDB is overkill for a single string. A cookie is meaningless on
`file://`.

**Alternatives considered**:
- `sessionStorage`: rejected — defeats the cross-session "remember me"
  goal.
- A cookie or query-string parameter: rejected — file:// origins do
  not behave as expected for cookies, and query params would require
  re-emitting the report.
- No persistence (always type fresh): rejected — explicit user
  decision that persistence is required.

## R5: Persistence trigger

**Decision**: Persist on definitive worker submit success
(`renderSuccess` path), not on every keystroke and not on
`closeDialog`.

**Rationale**: Mirrors the "definitive logical attempt" boundary the
existing code already uses for `idempotencyKey` lifecycle. Persisting
on every keystroke pollutes localStorage with abandoned drafts.
Persisting on `closeDialog` would memorise an Author the user typed
but explicitly cancelled (e.g. typed "test", changed their mind, hit
Esc). Persisting on success guarantees the stored name was actually
attached to a real GitHub issue.

**Alternatives considered**:
- On every input event: rejected — pollutes storage with abandoned
  drafts, including impersonation typos (user starts typing one name,
  changes it).
- On dialog open with the field pre-cached: not applicable; we want
  the value at submit time, not at open time.
- On `closeDialog`: rejected — captures cancelled drafts.

## R6: Pre-fill timing

**Decision**: Read `localStorage.getItem('mygather.feedback.lastAuthor')`
inside `openDialog`, and (a) if non-empty and within `authorMaxChars`,
assign it to `authorInput.value`; (b) otherwise leave the input empty.
Read on EACH open (not once at boot) so a value persisted in a prior
dialog cycle within the same page session is visible on the next
open.

**Rationale**: `closeDialog` clears every form field today; reading at
open time matches that lifecycle and avoids "the input was cleared
between open events" surprises. Validating against `authorMaxChars`
defends against a stale localStorage entry left by a future build with
a higher cap; we silently truncate a too-long persisted value to the
current cap rather than throwing or leaving the Submit button stuck
disabled.

**Alternatives considered**:
- Pre-fill once at boot: would not survive `closeDialog`'s
  `authorInput.value = ""` reset.
- Pre-fill at `closeDialog` time: would race with the user reopening.

## R7: Worker contract version compatibility

**Decision**: Forward-only contract change. After this PR ships, an
older report binary that does not POST `author` is rejected by the
worker with 400 `author_required`. There is no back-compat shim.

**Rationale**: Principle XIII forbids fallbacks for an internal
contract field. The graceful degradation path is the existing
`window.open` GitHub fallback that the older binary's frontend
already routes to on a 4xx response. Users with an old binary will
see the GitHub URL pop open (with no `Submitted by:` line — that
binary doesn't know about Author), which is the documented
non-network fallback. Once they refresh the report binary, the
worker path resumes.

**Alternatives considered**:
- Worker treats missing `author` as legacy and synthesises
  `Submitted by: (unknown)`: rejected — would be a silent fallback
  (Principle XIII), and would mask a real client bug after rollout.
- Bump the contract URL/version: rejected — no precedent in the
  existing codebase, and the named-exception POST URL is a build-time
  constant; adding a versioned alias path in the worker would be a
  parallel route (Principle XIII).
