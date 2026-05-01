# Feature Specification: Spec And Code Reconciliation

**Feature Branch**: `009-spec-code-reconciliation`
**Created**: 2026-05-01
**Status**: Complete
**Input**: User description: "Create a new branch and PR that fixes all spec/code drift found by the deep analysis so the repository is working and spec/code are correct."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Trust Spec Status At A Glance (Priority: P1)

A maintainer opens the repository on `main` or a reconciliation branch and needs
the active feature pointer, agent context files, and feature status lines to
match the actual implementation state.

**Why this priority**: Stale feature pointers and `Draft` statuses cause Spec
Kit commands to fail or send future work to the wrong feature.

**Independent Test**: Inspect `.specify/feature.json`, `AGENTS.md`,
`CLAUDE.md`, and every `specs/*/spec.md` status line. Completed features with
fully checked task lists are not left as `Draft`, and the active feature points
to this reconciliation work while the branch is active.

**Acceptance Scenarios**:

1. **Given** the current branch is `009-spec-code-reconciliation`, **When** an
   agent reads the active feature block, **Then** it names
   `009-spec-code-reconciliation`.
2. **Given** a feature task list is fully checked, **When** the spec status is
   reviewed, **Then** the status says `Complete` unless the spec is explicitly
   historical or superseded.
3. **Given** a shipped or superseded historical feature has stale task tracking,
   **When** a maintainer opens its task file, **Then** the file clearly states
   whether it is authoritative or historical.

---

### User Story 2 - Archive Input Contracts Are Tested (Priority: P1)

A maintainer changes archive input behavior and needs tests for every
documented archive error branch.

**Why this priority**: The active feature `008-archive-input` added untrusted
archive handling. Missing tests for zero-root archives and unsupported regular
files leave documented CLI contracts underprotected.

**Independent Test**: Run `go test ./cmd/my-gather` and verify coverage exists
for archive input success, unsafe archive rejection, corrupt archive rejection,
multiple-root rejection, zero-root rejection, and unsupported regular input
files.

**Acceptance Scenarios**:

1. **Given** a supported archive contains no pt-stalk root, **When** the CLI is
   run against it, **Then** it exits with the documented not-a-pt-stalk code and
   a clear message.
2. **Given** a regular file uses an unsupported extension, **When** the CLI is
   run against it, **Then** it exits with the documented input-path code and
   names the supported archive formats.

---

### User Story 3 - Open Historical Gaps Are Explicit (Priority: P2)

A maintainer reviews older feature specs and needs to know which remaining
items are live release blockers, deferred follow-up, or historical context.

**Why this priority**: The project uses specs as operational memory. Ambiguous
open tasks can cause agents to reopen old work or claim an incomplete release.

**Independent Test**: Inspect the noted historical files for unresolved
placeholders, misleading stale task states, and release-gate wording that no
longer reflects current project flow.

**Acceptance Scenarios**:

1. **Given** feature 001 has a UX audit checklist sample row, **When** it is
   reviewed as a durable artifact, **Then** it does not contain an unresolved
   placeholder task ID.
2. **Given** feature 002 is superseded by feature 003, **When** its task file is
   reviewed, **Then** it does not imply 40 live unchecked tasks remain for the
   shipped product.

### Edge Cases

- Historical specifications may intentionally preserve original user wording;
  reconciliation must not rewrite exempt verbatim input or raw test fixtures.
- Active feature pointers must remain synchronized across `AGENTS.md`,
  `CLAUDE.md`, and `.specify/feature.json`.
- Code changes must avoid creating a second implementation path for archive
  input behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The repository MUST contain a reconciliation feature under
  `specs/009-spec-code-reconciliation/` with spec, plan, research, data model,
  quickstart, contracts, and tasks artifacts.
- **FR-002**: `AGENTS.md`, `CLAUDE.md`, and `.specify/feature.json` MUST point
  to the same active feature while this branch is active.
- **FR-003**: Specs for completed fully checked features MUST not remain marked
  `Draft`.
- **FR-004**: Historical or superseded task files MUST explicitly state that
  their checklist state is historical when it no longer maps one-to-one to the
  shipped implementation.
- **FR-005**: Archive input tests MUST cover supported archives with zero
  pt-stalk roots.
- **FR-006**: Archive input tests MUST cover unsupported regular files and
  assert the documented exit-code category plus supported-format guidance.
- **FR-007**: Reconciliation edits MUST preserve English-only durable artifacts
  and must not alter raw fixture or reference data.
- **FR-008**: Validation MUST include focused archive-input tests and the full Go
  test suite before the PR is opened.

### Key Entities

- **Feature Status Line**: The `**Status**` field in a feature specification.
- **Active Feature Pointer**: The synchronized references in `AGENTS.md`,
  `CLAUDE.md`, and `.specify/feature.json`.
- **Historical Task File**: A task list retained for traceability after the
  shipped behavior has been superseded or reconciled elsewhere.
- **Archive Error Test**: A focused CLI test that pins one documented archive
  error branch.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test ./cmd/my-gather` passes with tests for the two previously
  missing archive-input error branches.
- **SC-002**: `go test ./...` passes.
- **SC-003**: All active feature pointers name `009-spec-code-reconciliation`
  on the branch.
- **SC-004**: No feature with all tasks checked remains marked `Draft`.
- **SC-005**: No unresolved placeholder task ID remains in checked-in spec
  artifacts outside historical examples.

## Assumptions

- Feature 001 remains a broad historical MVP tracker with known deferred and
  open quality tasks, not a fully complete feature.
- Feature 002's unchecked task list is historical because its shipped feedback
  behavior was reconciled and superseded by feature 003.
- The archive-input implementation behavior is mostly correct; this feature
  adds missing test coverage and spec/task clarity rather than redesigning
  archive extraction.
