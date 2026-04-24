<!--
Sync Impact Report
==================
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

Every behaviour in My-gather MUST have exactly one implementation. When a
function, type, or code path is replaced, the old one MUST be deleted in
the same change — not left behind, not guarded by an internal flag, not
retained "for safety". Silent fallbacks (try A, on failure silently try B)
are prohibited; recoverable failures MUST surface as typed errors
(Principle VII) or structured diagnostics in the report (Principle III),
never as a second hidden attempt. Re-exports and compatibility shims for
internal identifiers after a rename are prohibited; all call sites MUST
be updated in the same commit. This principle does not forbid branching
driven by genuine input variation (e.g., distinct pt-stalk file format
versions) or platform primitives (`path/filepath`), provided the branches
converge into a single typed model as early as possible.

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

## Distribution & Platform Support

Release artifacts MUST be produced reproducibly from tagged commits and
MUST cover `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`
at minimum. Binaries MUST be built with `CGO_ENABLED=0` and stripped of
host-specific paths so that two builders producing the same tag obtain
byte-identical binaries (subject to Go toolchain determinism guarantees).
Shipped binaries MUST print a version string that includes the Go version,
the git commit, and the build date, sourced via `-ldflags` at link time.

## Development Workflow & Quality Gates

The following gates MUST pass before any change is merged:

1. `go vet ./...` and `go test ./...` succeed on all supported platforms
   exercised in CI.
2. Every new or modified parser ships with its fixture and golden file
   (Principle VIII).
3. Determinism check: a test re-runs report generation twice on the same
   fixture set and asserts byte-identical output (Principle IV).
4. No new direct dependency is added without a justification entry in the
   corresponding `plan.md` (Principle X).
5. Exported identifiers added or changed have godoc comments describing
   their contract (Principle VI).
6. No build introduces a CGO requirement, a network call at runtime, or a
   write under the input tree (Principles I, IX, II).
7. No change leaves a duplicated or fallback implementation of an existing
   behaviour in place (Principle XIII). Replaced functions, types, and
   code paths MUST be deleted in the same change; internal re-exports and
   compatibility shims after a rename are prohibited.
8. No change introduces non-English content into any checked-in artifact
   outside `testdata/` and `_references/` (Principle XIV). Source code,
   comments, commit messages, branch names, specs, docs, `.claude/` agent
   and skill definitions, and shell scripts MUST be English-only.

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

**Version**: 1.3.0 | **Ratified**: 2026-04-21 | **Last Amended**: 2026-04-24
