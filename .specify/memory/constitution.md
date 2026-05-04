<!--
Sync Impact Report
==================
Version change: 1.5.0 → 1.6.0
Bump rationale: MINOR-level material expansion of existing Principle
  XIII. The canonical-code-path rule already forbade duplicated
  implementations, silent fallbacks, and internal compatibility shims;
  this amendment broadens the wording to match the full project
  surface (features, workflows, APIs, helpers, contracts, worker
  routes, UI behaviours, and review skills) and adds explicit
  external-degradation criteria so legitimate graceful degradation
  cannot be confused with forbidden internal fallback paths. No new
  principle is added.

Added principles: none.

Modified principles:
  - XIII. Canonical Code Path (NON-NEGOTIABLE) → broadened from
    function/type/code-path wording to every maintained behaviour,
    workflow, API, helper, parser, validator, renderer path, worker
    route, UI behaviour, and review skill; added observable/tested
    external-degradation criteria and a requirement that specs/plans/
    tasks identify the canonical path for touched behaviour.

Modified sections:
  - Development Workflow & Quality Gates → gate 7 now names the
    expanded canonical-path audit expectations for specs, plans, tasks,
    and review.

Templates requiring updates:
  - .specify/templates/plan-template.md             updated
  - .specify/templates/tasks-template.md            updated
  - .specify/templates/spec-template.md             updated
  - .specify/templates/checklist-template.md        compatible
  - .claude/agents/pre-review-constitution-guard.md updated
  - .claude/skills/pr-review-*-my-gather/           updated
  - AGENTS.md                                      compatible
  - CLAUDE.md                                      updated
  - README.md                                      updated

Deferred items / follow-up TODOs:
  - Existing source-code canonical-path smells identified during the
    audit (for example render-side helper duplication and legacy
    fallback/migration comments) are intentionally not changed in this
    documentation-only amendment. They should be handled in a separate
    source refactor.

Prior Sync Impact Report (1.5.0) follows for history:
-----------------------------------------------------
Version change: 1.4.0 → 1.5.0
Bump rationale: MINOR-level addition of a new Core Principle — XV.
  Bounded Source File Size — plus a companion mechanical quality gate.
  The rule forbids first-party source-code files over 1000 lines so
  god files are blocked before review. It is scoped intentionally to
  source code, not specs, docs, JSON data, raw pt-stalk fixtures,
  reference captures, generated lockfiles, or golden snapshots. The
  repository is brought into compliance in the same PR by splitting
  the maintained report JS/CSS assets into ordered embedded source
  parts.

Added principles:
  - XV. Bounded Source File Size

Modified sections:
  - Development Workflow & Quality Gates → added 9th merge gate:
      "No governed first-party source-code file exceeds 1000 lines
      (Principle XV)."

Templates requiring updates:
  - .specify/templates/plan-template.md             compatible
  - .specify/templates/spec-template.md             compatible
  - .specify/templates/tasks-template.md            compatible
  - .specify/templates/checklist-template.md        compatible
  - .claude/skills/speckit-*/                       compatible
  - .agents/skills/speckit-*/                       compatible

Deferred items / follow-up TODOs: none.

Prior Sync Impact Report (1.4.0) follows for history:
-----------------------------------------------------
Version change: 1.3.1 → 1.4.0
Bump rationale: MINOR-level mechanisation of two previously REVIEW-
  only gates. Gates 5 (Principle VI godoc-on-exports) and 8
  (Principle XIV English-only) now flip from [REVIEW] to
  [MECHANICAL]. Their forbidden / required behaviours are
  unchanged - the principles still demand exactly what they
  demanded yesterday - but a regression that previously slipped
  through to PR review will now fail CI or the pre-push hook
  before reaching origin. Nothing already-compliant becomes
  non-compliant; this is a strict tightening of mechanical teeth
  on existing principles, hence MINOR (per the Governance
  versioning policy: "MINOR: A new principle or section is added,
  or existing guidance is materially expanded").

  Concrete checks landed in this PR (chore/pr32-mechanise-review-
  gates):
    - Gate 5: tests/coverage/godoc_coverage_test.go AST-walks
      parse/, model/, render/, findings/ and asserts every
      exported identifier has a non-empty doc comment. CI invokes
      it via `go test ./tests/coverage/... -run TestGodocCoverage`
      in the existing matrix `test` job. The pre-push hook runs
      the same target locally when the push touches Go files
      under those packages.
    - Gate 8: scripts/hooks/pre-push-constitution-guard.sh greps
      every changed file outside testdata/ and _references/ for
      UTF-8 byte sequences that encode Latin-1 Supplement letters
      (00C0-00FF, with U+00D7 multiplication and U+00F7 division
      carved out) and Latin Extended-A (0100-017F). Math symbols,
      em-dashes, arrows, and emoji pass; Spanish / French / German
      / etc. accented letters do not. Two pre-existing legitimate
      uses are exempt by file:line.
    - Pre-push hook scope expansion (companion teeth on existing
      MECHANICAL halves of gate 6): suspicious build tags
      (Principles I, XII), `net/http` import or net.Dial in the
      Go release path (Principle IX), and writes inside the
      input-reading path (Principle II) now block the push.

Added principles: none.

Modified principles: none.

Modified sections:
  - Development Workflow & Quality Gates -> gates 5 and 8 flip from
    [REVIEW] to [MECHANICAL] and gain a one-line description of
    the new check. Gate 6 also flips from [MECHANICAL - partial]
    to [MECHANICAL] now that the pre-push hook covers all three
    of its sub-rules (CGO, network in release path, writes in
    input-reading path) - the new sub-rules land in the same PR
    that flips gate 5 and gate 8. The intro paragraph keeps its
    mixed-tag convention sentence as a forward-compatible note;
    no current gate uses the partial form. The closing paragraph's
    "future direction" sentence is rewritten to mention only gate
    7 (canonical code path) as the outstanding REVIEW gate that
    future amendments may convert.

Downstream cleanups folded into this amendment: none. The
  mechanisation lands alongside the wording.

Templates requiring updates:
  - .specify/templates/plan-template.md             compatible
  - .specify/templates/spec-template.md             compatible
  - .specify/templates/tasks-template.md            compatible
  - .specify/templates/checklist-template.md        compatible
  - .claude/skills/speckit-*/                       compatible
  - .claude/skills/pr-review-*-my-gather/           compatible
    (skills reference the constitution by file path, not by gate
    tag wording or version)
  - .claude/agents/pre-review-constitution-guard.md compatible
    (the agent's principle-walk does not depend on gate-tag prose
    or the MECHANICAL / REVIEW split)

Deferred items / follow-up TODOs: gate 7 (Principle XIII canonical
  code path) remains REVIEW. A duplicate-implementation scan
  needs more design work (a static-analysis pass or AST diff
  rather than a regex) and is deferred to a future amendment.

Prior Sync Impact Report (1.3.1) follows for history:
-----------------------------------------------------
Version change: 1.3.0 → 1.3.1
Bump rationale: PATCH-level wording clarification on the Development
  Workflow & Quality Gates section. Each of the 8 merge gates is now
  labelled [MECHANICAL] (enforced by CI or the pre-push hook) or
  [REVIEW] (enforced by human review against the constitution). No
  forbidden / required behaviour changes; nothing already-compliant
  becomes non-compliant. The labels make it honest which gates the
  repo enforces with code today vs. which rely on reviewer judgement,
  surfacing where a future tooling investment (e.g., extending
  scripts/hooks/pre-push-constitution-guard.sh) would close the gap.
  Audit on main @ 1cc3459 (chore/spec-audit-reconciliation, 2026-04-27)
  drove the clarification.

Added principles: none.

Modified principles: none.

Modified sections:
  - Development Workflow & Quality Gates → each of the 8 gates gains
    a [MECHANICAL] / [REVIEW] tag in its leading line. Intro paragraph
    above the list adds one sentence explaining the tag convention.
    Gate text is unchanged.

Downstream cleanups folded into this amendment: none.

Templates requiring updates:
  - .specify/templates/plan-template.md      ✅ compatible
  - .specify/templates/spec-template.md      ✅ compatible
  - .specify/templates/tasks-template.md     ✅ compatible
  - .specify/templates/checklist-template.md ✅ compatible
  - .claude/skills/speckit-*/                ✅ compatible
  - .claude/agents/pre-review-constitution-guard.md ✅ compatible
    (the agent's principle-walk does not depend on gate-tag prose)

Deferred items / follow-up TODOs: none. Future amendment may extend
  pre-push hook + CI lint coverage to convert REVIEW gates into
  MECHANICAL gates (godoc on exports, canonical-path duplicate scan,
  English-only ASCII grep) — that would be its own MINOR amendment
  with the new mechanical checks landing alongside the wording.

Prior Sync Impact Report (1.3.0) follows for history:
-----------------------------------------------------
Version change: 1.2.0 → 1.3.0
Bump rationale: MINOR-level named exception added to Principle IX
  (Zero Network at Runtime). The exception permits a single outbound
  POST to a project-controlled endpoint on explicit user Submit of
  the Report Feedback dialog, enabling feature 003-feedback-backend-worker
  to hand the feedback off to a Cloudflare Worker that posts the
  GitHub Issue server-side (the only path that avoids shipping a
  token in the report). The broader no-network baseline is unchanged;
  nothing already-compliant becomes non-compliant.

Added principles: none.

Modified principles:
  - IX. Zero Network at Runtime → adds a "Named exceptions" subsection
    describing the user-initiated-Submit path. Baseline rule
    ("MUST NOT perform any network I/O during normal operation") is
    preserved verbatim. Only additive text.

Modified sections: none outside the principle body itself.

Downstream cleanups folded into this amendment:
  - None. The amendment lands alongside the feature that needs it,
    per plan.md.

Templates requiring updates:
  - .specify/templates/plan-template.md      ✅ compatible
  - .specify/templates/spec-template.md      ✅ compatible
  - .specify/templates/tasks-template.md     ✅ compatible
  - .specify/templates/checklist-template.md ✅ compatible
  - .claude/skills/speckit-*/                ✅ compatible

Deferred items / follow-up TODOs: none.

Prior Sync Impact Report (1.2.0) follows for history:
-----------------------------------------------------
Version change: 1.1.0 → 1.2.0
Bump rationale: MINOR-level addition of a new Core Principle — XIV.
  English-Only Durable Artifacts — and a companion 8th entry in the
  Development Workflow & Quality Gates section. The principle
  codifies the pre-existing CLAUDE.md "Language convention" rule at
  constitution level so reviewers, the pre-review-constitution-guard
  agent, and the /pr-review-fix-my-gather skill all gate on it. It
  explicitly exempts content under `testdata/` and `_references/`
  (raw pt-stalk input, preserved by Principle II) and out-of-band
  chat with the user. No existing principle is weakened or
  redefined; this is additive, hence MINOR rather than MAJOR.

Added principles:
  - XIV. English-Only Durable Artifacts

Modified sections:
  - Development Workflow & Quality Gates → added 8th merge gate:
      "No non-English content is introduced into any checked-in
      artifact outside `testdata/` and `_references/` (Principle XIV)."

Downstream cleanups folded into this amendment:
  - .claude/skills/pr-review-fix-my-gather/SKILL.md → removed the
      Spanish <example> block from the description (artifact MUST
      be English under XIV).
  - CLAUDE.md → replaced the 13-line "Language convention" block
      with a one-line pointer to Principle XIV, eliminating the
      duplicated authority.

Templates requiring updates:
  - .specify/templates/plan-template.md      ✅ compatible
  - .specify/templates/spec-template.md      ✅ compatible
  - .specify/templates/tasks-template.md     ✅ compatible
  - .specify/templates/checklist-template.md ✅ compatible
  - .claude/skills/speckit-*/                ✅ compatible
  - README.md                                ⚠ pending (update when
      README documents principles explicitly)

Deferred items / follow-up TODOs:
  - Consider a mechanical English-only grep in
      scripts/hooks/pre-push-constitution-guard.sh once drift is
      observed. Skip until a real violation shows up — start with
      governance teeth, add mechanical enforcement only if needed.

Prior Sync Impact Report (1.1.0) follows for history:
-----------------------------------------------------
Version change: 1.0.1 → 1.1.0
Bump rationale: MINOR-level addition of a new Core Principle — XIII.
  Canonical Code Path (NON-NEGOTIABLE) — and a companion 7th entry in
  the Development Workflow & Quality Gates section enforcing it at
  merge time. The principle forbids duplicated implementations, silent
  fallbacks, and compatibility shims for internal identifiers, while
  preserving legitimate branching at true system boundaries (distinct
  pt-stalk file format versions, platform primitives) and structured
  diagnostic reporting under Principle III. No existing principle is
  weakened or redefined; this is additive, hence MINOR rather than
  MAJOR.

Added principles:
  - XIII. Canonical Code Path (NON-NEGOTIABLE)

Modified sections:
  - Development Workflow & Quality Gates → added 7th merge gate:
      "No change leaves a duplicated or fallback implementation of an
      existing behaviour in place (Principle XIII)."

Templates requiring updates:
  - .specify/templates/plan-template.md      ✅ compatible (generic
      Constitution Check gate; no principle names hard-coded)
  - .specify/templates/spec-template.md      ✅ compatible
  - .specify/templates/tasks-template.md     ✅ compatible
  - .specify/templates/checklist-template.md ✅ compatible
  - .claude/skills/speckit-*/                ✅ compatible (reference
      the constitution file path, not principle names)
  - README.md                                ⚠ pending (update when
      README documents principles explicitly)

Deferred items / follow-up TODOs: none.

Prior Sync Impact Report (1.0.1) follows for history:
-----------------------------------------------------
Version change: 1.0.0 → 1.0.1
Bump rationale: PATCH-level wording fix. Principle VIII referenced the
  fixture source path as `references/examples/`; the repository actually
  uses `_references/examples/` (leading underscore marks the directory
  as non-shipped reference material). No forbidden/required behaviours
  change — only the path string is corrected. Closes analyze finding
  F12.

Modified principles:
  - VIII. Reference Fixtures & Golden Tests → path string corrected
      from `references/examples/` to `_references/examples/`.

Templates requiring updates: none (no template hard-codes this path).

Prior Sync Impact Report (1.0.0) follows for history:
----------------------------------------------------
Version change: (uninitialized template) → 1.0.0
Bump rationale: Initial ratification of the My-gather project constitution.
  Prior file contained only placeholder tokens; this is the first concrete
  adoption, so MAJOR=1, MINOR=0, PATCH=0.

Modified principles:
  - [PRINCIPLE_1_NAME] → I. Single Static Binary
  - [PRINCIPLE_2_NAME] → II. Read-Only Inputs
  - [PRINCIPLE_3_NAME] → III. Graceful Degradation (NON-NEGOTIABLE)
  - [PRINCIPLE_4_NAME] → IV. Deterministic Output
  - [PRINCIPLE_5_NAME] → V. Self-Contained HTML Reports
Added principles (beyond the 5-slot template):
  - VI. Library-First Architecture
  - VII. Typed Errors
  - VIII. Reference Fixtures & Golden Tests
  - IX. Zero Network at Runtime
  - X. Minimal Dependencies
  - XI. Reports Optimized for Humans Under Pressure
  - XII. Pinned Go Version

Added sections:
  - Distribution & Platform Support (replaces [SECTION_2_NAME])
  - Development Workflow & Quality Gates (replaces [SECTION_3_NAME])

Removed sections: none.

Templates requiring updates:
  - .specify/templates/plan-template.md         ✅ compatible (generic
      "Constitution Check" gate; no principle names hard-coded)
  - .specify/templates/spec-template.md         ✅ compatible (no
      constitution references; no change required)
  - .specify/templates/tasks-template.md        ✅ compatible (no
      constitution references; no change required)
  - .specify/templates/checklist-template.md    ✅ compatible (no
      constitution references; no change required)
  - .claude/skills/speckit-*/                   ✅ compatible (command
      prompts reference the constitution file path, not principle names)
  - README.md                                   ⚠ pending (no README
      present in repo; create when project documentation is added and
      link to this constitution)

Deferred items / follow-up TODOs: none.
-->

# My-gather Constitution

## Core Principles

### I. Single Static Binary

My-gather MUST ship as one statically-linked executable with no runtime
dependencies. A support engineer MUST be able to `scp` the binary to an
arbitrary customer jump host and run it with no installer, no interpreter,
and no libc version negotiation. Release artifacts MUST cover, at minimum,
`linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`, built via
`CGO_ENABLED=0` cross-compilation. Any change that would reintroduce dynamic
linkage or a runtime install step is forbidden without a constitution
amendment.

### II. Read-Only Inputs

My-gather MUST NOT modify, move, rename, delete, or otherwise alter any
file or directory under the pt-stalk output tree it is pointed at. All
parsers MUST open inputs read-only. Any code path that could write inside
the input tree (including tempfile placement, lock files, or cache
writes) is prohibited; temporary artifacts MUST live under `os.TempDir()`
or an explicit `-out` path chosen by the user.

### III. Graceful Degradation (NON-NEGOTIABLE)

Real-world pt-stalk collections are routinely incomplete, truncated, or
corrupt. My-gather MUST produce a useful report from whatever input is
present and MUST clearly mark what is missing or unparseable in the
rendered output. The tool MUST NOT `panic` on malformed inputs and MUST
NOT silently drop data. Parser failures MUST be captured as structured
diagnostics attached to the report, not propagated as fatal errors; only
structural preconditions (e.g., target path does not exist, user lacks
read permission) justify early termination.

### IV. Deterministic Output

Given the same input directory and the same binary, My-gather MUST produce
byte-identical HTML output on every run, modulo timestamps explicitly
rendered from input data. Sources of non-determinism that would leak into
output — map iteration order, goroutine scheduling, unstable sort, locale-
dependent or default floating-point formatting, randomised IDs — MUST be
eliminated at the boundary (sorted keys, stable orderings, fixed-precision
number formatting, deterministic ID generation). Golden tests enforce this
invariant.

### V. Self-Contained HTML Reports

The generated report MUST be a single `.html` file with all CSS, JavaScript,
fonts, images, and underlying data embedded inline. Chart libraries and any
other client-side assets MUST be bundled into the binary via `//go:embed`
and inlined at generation time. Reports MUST NOT fetch from CDNs, external
fonts services, analytics endpoints, or any remote resource at view-time;
they MUST render fully on an air-gapped laptop with no network.

### VI. Library-First Architecture

The CLI (`package main`) MUST be a thin wrapper over independently
importable packages: `parse/` for pt-stalk file parsers, `model/` for the
typed in-memory representation, and `render/` for HTML generation. Each
package MUST be usable by third-party Go code without depending on `main`,
without reading CLI flags, and without writing to stdout/stderr except via
caller-supplied interfaces. Every exported identifier MUST carry a godoc
comment describing its contract, and that contract MUST be treated as
stable under the project's versioning policy.

### VII. Typed Errors

Conditions that a caller needs to branch on MUST be expressed as sentinel
errors or typed error structs — e.g., `var ErrNotAPtStalkDir = errors.New(...)`
or `type ParseError struct { File string; Line int; Err error }` — so
callers can use `errors.Is` and `errors.As`. Using `fmt.Errorf` (without
`%w` wrapping) to signal a branchable condition is prohibited. Error
messages intended only for human display MAY use `fmt.Errorf` but MUST
still wrap underlying typed errors with `%w` when one exists.

### VIII. Reference Fixtures & Golden Tests

For every pt-stalk collector file format My-gather supports, there MUST be
at least one fixture under `testdata/` drawn from real samples in
`_references/examples/`, and a round-trip test that parses the fixture and
compares rendered output against a committed `.golden` file. Adding a new
collector parser without a fixture and golden test is forbidden. Golden
files MUST be regenerated only through an explicit, reviewed step (e.g.,
`go test -update`), never silently.

### IX. Zero Network at Runtime

My-gather MUST NOT perform any network I/O during normal operation: no
telemetry, no crash reporting, no update checks, no DNS lookups, no
outbound HTTP. Network code paths (e.g., `net/http` client usage) are
prohibited in the shipped binary except behind build tags used exclusively
for development tooling that is NOT compiled into release artifacts.

**Named exceptions**: The following runtime network calls are allowed
when the user has an explicit intent expressed by a UI action:

- **Feedback submission (ratified 2026-04-24 for feature
  003-feedback-backend-worker)**: On the user clicking the "Submit"
  button of the Report Feedback dialog, the report MAY perform a
  single outbound `POST` to a project-controlled HTTPS endpoint. The
  endpoint URL MUST be a build-time constant. The payload MUST be
  scoped to the user's explicit feedback (title, body, attachments);
  it MUST NOT include telemetry, report metadata beyond the feedback
  itself, or any data the user did not type into the dialog. On any
  failure of the endpoint, the report MUST fall back to a non-network
  path that delivers equivalent value (feature 002's `window.open`
  pre-fill URL flow satisfies this). The Go binary itself remains
  subject to the baseline rule above — it performs no network I/O.

### X. Minimal Dependencies

The standard library is the default. Third-party modules MUST be justified
in the relevant plan document, with a stated reason why the stdlib is
inadequate. Framework-scale imports for problems the stdlib handles
directly (flag parsing, HTML templating, JSON encoding, time handling, HTTP
serving for local preview) are prohibited. Transitive dependency growth is
a review concern: each new direct dependency MUST list its transitive
footprint in the plan.

### XI. Reports Optimized for Humans Under Pressure

Reports MUST prioritise the 80% signal a support engineer needs during an
incident: what triggered the collection, what the system looked like at
trigger time, and what changed across samples. Exhaustive metric dumps
MUST be relegated to collapsible or secondary sections and MUST NOT crowd
out the primary narrative. Any new top-level report section MUST justify
its placement against this principle.

### XII. Pinned Go Version

My-gather MUST target the current Go stable release. The required version
MUST be pinned in `go.mod` via the `go` directive and enforced in CI. Build
tags that diverge runtime behaviour between platforms are prohibited
except where genuinely unavoidable (e.g., path separator handling, which
MUST go through `path/filepath`). Upgrading the Go version MUST be
performed as an explicit, reviewed change, not as an incidental side effect
of another commit.

### XIII. Canonical Code Path (NON-NEGOTIABLE)

Every behaviour, feature, workflow, API, helper, parser, validator,
renderer path, worker route, UI behaviour, review skill, and automation
entry point in My-gather MUST have exactly one canonical implementation
path. New code MUST reuse, move, or improve that canonical path instead
of creating a parallel helper, wrapper, duplicate API, compatibility shim,
internal feature flag, or competing implementation. When a function, type,
contract, file layout, or code path is replaced, the old path MUST be
deleted in the same change — not left behind, not guarded by an internal
flag, not retained "for safety". Re-exports and compatibility shims for
internal identifiers after a rename are prohibited; all call sites MUST be
updated in the same commit.

Silent internal fallbacks (try A, on failure silently try B) are prohibited.
Recoverable failures MUST surface as typed errors (Principle VII) or
structured diagnostics in the report (Principle III), never as a second
hidden attempt. External degradation paths are allowed only for genuine
runtime boundaries outside the canonical implementation's control, such as
missing or malformed pt-stalk inputs, unavailable browser capabilities,
network or upstream-service failure on the named feedback exception, or
platform primitives (`path/filepath`). Such degradation MUST be observable
to the user or caller, covered by tests or explicit review evidence, and
routed through the canonical owner rather than a preserved old
implementation.

This principle does not forbid branching driven by genuine input variation
(e.g., distinct pt-stalk file format versions), provided the branches
converge into a single typed model as early as possible. Specs, plans, and
tasks that touch existing behaviour MUST identify the canonical owner/path,
state whether an old path is removed or unchanged, and include a review
step that verifies no duplicate implementation, hidden fallback, or
compatibility shim remains.

### XIV. English-Only Durable Artifacts

All artifacts checked into this repository MUST be written in English.
This covers source code (identifiers, comments, godoc), commit messages,
branch names, tags, pull-request titles and descriptions, pull-request
and issue review comments, GitHub issues opened from this repo, and
every file under `specs/`, `.specify/`, `docs/`, `.claude/` (agent
definitions, skill descriptions and their example trigger phrases,
settings), `scripts/` (including hook scripts), `README.md`, and
`CHANGELOG.md`. The following are exempt: (a) content under `testdata/`
and `_references/`, which reproduces raw pt-stalk input and MUST NOT be
modified (Principle II); (b) out-of-band chat with the user via the
Claude Code TUI or another interactive channel, which is not a
checked-in artifact. A change that introduces non-English text into any
other artifact MUST be rejected unless the same pull request carries a
constitution amendment adopting a named exception.

### XV. Bounded Source File Size

First-party source-code files MUST NOT exceed 1000 lines. This is a
maintainability boundary against god files: when a source file grows past the
limit, the change MUST split it by responsibility before merge. Governed
source-code files include Go, JavaScript, TypeScript, CSS, shell, HTML template,
and similar maintained implementation files. The rule does not apply to specs,
docs, JSON data, raw pt-stalk fixtures under `testdata/`, reference captures
under `_references/`, committed golden snapshots, generated dependency
lockfiles, or vendored third-party/minified assets. Exemptions for maintained
first-party source code are prohibited without a constitution amendment naming
the exception and its removal plan.

## Distribution & Platform Support

Release artifacts MUST be produced reproducibly from tagged commits and
MUST cover `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`
at minimum. Binaries MUST be built with `CGO_ENABLED=0` and stripped of
host-specific paths so that two builders producing the same tag obtain
byte-identical binaries (subject to Go toolchain determinism guarantees).
Shipped binaries MUST print a version string that includes the Go version,
the git commit, and the build date, sourced via `-ldflags` at link time.

## Development Workflow & Quality Gates

The following gates MUST pass before any change is merged. Each gate is
tagged **[MECHANICAL]** (enforced by CI in `.github/workflows/` or by
`scripts/hooks/pre-push-constitution-guard.sh`, blocking the change
without human action) or **[REVIEW]** (enforced by human review against
this constitution; the repo currently has no mechanical check that
catches a violation). A gate may carry a partial / mixed tag
(e.g. **[MECHANICAL — partial]** combined with an inline **[REVIEW]**
on the un-mechanised sub-rules) when only one half of its scope is
caught by tooling. The convention is preserved for use by future
gates that land in a mixed state. The tag tells you whether you can
rely on tooling to catch a regression, or whether reviewer attention
is the only line of defence:

1. **[MECHANICAL]** `go vet ./...` and `go test ./...` succeed on all
   supported platforms exercised in CI.
2. **[MECHANICAL]** Every new or modified parser ships with its fixture
   and golden file (Principle VIII). New collector parsers are caught by
   `scripts/hooks/pre-push-constitution-guard.sh` (the `COLLECTOR_PARSERS`
   list); behavioural changes to existing parsers are mechanically caught
   by CI when their golden tests fail unless the golden is updated, with
   review covering the remaining governance expectation (intentional
   vs. accidental output drift, fixture-coverage of new code paths).
3. **[MECHANICAL]** Determinism check: a test re-runs report generation
   twice on the same fixture set and asserts byte-identical output
   (Principle IV). The CI determinism job allows up to two diff lines —
   the `Report generated at` timestamp is the only legal drift.
4. **[MECHANICAL]** No new direct dependency is added without a
   justification entry in the corresponding `plan.md` (Principle X). The
   pre-push hook scans new `go.mod require` entries for a matching
   citation.
5. **[MECHANICAL]** Exported identifiers added or changed have godoc
   comments describing their contract (Principle VI). Enforced by
   `tests/coverage/godoc_coverage_test.go`, an AST-walking test that
   asserts every exported identifier under `parse/`, `model/`,
   `render/`, and `findings/` carries a non-empty doc comment. CI
   invokes it via `go test ./tests/coverage/... -run TestGodocCoverage`
   in the matrix `test` job, and `scripts/hooks/pre-push-constitution-
   guard.sh` runs the same target locally when the push touches Go
   files under those packages.
6. **[MECHANICAL]** No build introduces a CGO requirement (Principle I,
   pre-push hook scans for `import "C"` and for `//go:build cgo`),
   no new `net/http`, `net.Dial`, or `http.Get` / `http.Post` /
   `http.NewRequest` / `http.Client{}` lands in `cmd/`, `parse/`,
   `model/`, `render/`, or `findings/` (Principle IX, pre-push hook
   greps the diff), and no `os.Create` / `os.Mkdir` / `os.MkdirAll` /
   `os.Rename` / `os.Remove` / `os.RemoveAll` / `os.WriteFile` /
   `ioutil.WriteFile` lands in `parse/` or `cmd/` (Principle II,
   pre-push hook greps the diff). All three halves now block the
   push without human action.
7. **[REVIEW]** No change leaves a duplicated, competing, or fallback
   implementation of an existing behaviour in place (Principle XIII).
   Replaced functions, types, contracts, file layouts, workflows, APIs,
   helpers, worker routes, UI behaviours, review skills, and automation
   paths MUST delete the old path in the same change unless a separate
   constitution amendment names a temporary exception and removal plan.
   Specs, plans, tasks, and review notes MUST identify the canonical
   owner/path for touched behaviour and verify that any external
   degradation path is observable, tested or explicitly reviewed, and
   routed through that owner. Reviewers verify on the diff; no mechanical
   check today.
8. **[MECHANICAL]** No change introduces non-English content into any
   checked-in artifact outside `testdata/` and `_references/`
   (Principle XIV). Enforced by a byte-level grep in
   `scripts/hooks/pre-push-constitution-guard.sh` that catches
   UTF-8 sequences encoding Latin-1 Supplement letters
   (00C0–00FF, with U+00D7 multiplication and U+00F7 division
   carved out) and Latin Extended-A (0100–017F). Math symbols,
   em-dashes, arrows, and emoji pass; Spanish / French / German /
   etc. accented letters do not. Two pre-existing legitimate uses
   (the verbatim user-input quote at
   `specs/003-feedback-backend-worker/spec.md:6` and the UTF-8
   byte-counting test fixture at
   `feedback-worker/test/validate.test.ts:69-70`) are exempt by
   file:line.
9. **[MECHANICAL]** No governed first-party source-code file exceeds
   1000 lines (Principle XV). Enforced by
   `tests/coverage/file_size_test.go`, which scans tracked source files
   and fails with every offending path and line count. Raw fixtures,
   references, specs, docs, JSON data, generated dependency lockfiles,
   golden snapshots, and vendored third-party/minified assets are outside
   this source-code rule.

The MECHANICAL / REVIEW split documents what the repo enforces with code
today, not the strictness of each gate — every gate is equally
non-negotiable for a merge. As of v1.5 only gate 7 (Principle XIII
canonical code path) remains [REVIEW]; a future amendment may convert
it once a duplicate-implementation scan is designed. The constitution
does not require that conversion, but the absence of mechanical teeth
on the one remaining REVIEW gate is a useful signal for reviewers
reading a diff that touches existing behaviour.

Reviewers MUST reject changes that violate any Core Principle unless the
change is accompanied by a constitution amendment adopted under the
Governance section below.

## Governance

This constitution supersedes other ad-hoc practices and conventions in the
My-gather project. Amendments MUST be proposed as a pull request that:

- Modifies `.specify/memory/constitution.md` with the new text.
- Updates the `Version`, `Ratified`, and `Last Amended` line per the
  versioning policy.
- Includes a Sync Impact Report (as an HTML comment at the top of this
  file) describing the delta and any template or documentation updates
  propagated alongside the amendment.

Versioning policy for this constitution follows semantic versioning:

- **MAJOR**: A Core Principle is removed, redefined in a backward-
  incompatible way, or a governance rule is materially weakened.
- **MINOR**: A new principle or section is added, or existing guidance is
  materially expanded.
- **PATCH**: Wording clarifications, typo fixes, or non-semantic
  refinements that do not change the set of forbidden/required behaviours.

Compliance MUST be verified at every `/speckit.plan` and `/speckit.analyze`
invocation via the Constitution Check gate. Runtime development guidance
— build commands, test commands, repository layout — lives in `README.md`
and feature-local `plan.md` / `quickstart.md` files and MUST defer to this
constitution when conflicts arise.

**Version**: 1.6.0 | **Ratified**: 2026-04-21 | **Last Amended**: 2026-05-04
