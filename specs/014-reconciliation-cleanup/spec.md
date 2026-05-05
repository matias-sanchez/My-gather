# Feature Specification: Reconciliation Cleanup

**Feature Branch**: `014-reconciliation-cleanup`
**Created**: 2026-05-05
**Status**: Complete
**Input**: User description: "new round of reconciliation, open a new PR in a new branch, and run multi-agents"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintainer Sees Remaining Audit Findings Closed (Priority: P1)

A maintainer reviews the post-PR #47 audit and sees every confirmed remaining
finding either fixed or explicitly documented as a non-violation.

**Why this priority**: The previous audit improved the repository but still
found confirmed drift in parser diagnostics, Spec Kit automation, and feature
001 documentation.

**Independent Test**: Run the validation commands in `quickstart.md` and verify
the audit contract records no unresolved confirmed finding.

**Acceptance Scenarios**:

1. **Given** malformed non-numeric or negative netstat `TS` headers, **When**
   parsing runs, **Then** the parser emits a structured diagnostic and keeps
   sample boundaries deterministic.
2. **Given** Spec Kit git feature creation runs, **When** common helpers are
   loaded, **Then** there is one canonical helper source and no hidden fallback
   helper path.
3. **Given** feature 001 artifacts are reviewed, **When** current shipped
   behavior is referenced, **Then** section count, diagnostics, Go version, and
   constitution-principle references are not contradictory.

### User Story 2 - Reviewers Can Reproduce the Reconciliation (Priority: P2)

A reviewer can follow the feature artifacts and validation log without needing
prior chat context.

**Why this priority**: The reconciliation is audit-derived; the evidence must
be reviewable in the repository.

**Independent Test**: Inspect `contracts/audit.md`, `tasks.md`, and the PR body
for a complete finding-to-fix mapping.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The branch MUST include a feature artifact set for the
  reconciliation round.
- **FR-002**: Netstat and netstat_s parsing MUST diagnose malformed `TS`
  headers even when the epoch token is non-numeric or negative.
- **FR-003**: Spec Kit git feature-creation scripts MUST use one canonical
  helper source per shell family and MUST NOT keep fallback helper
  implementations for the same behavior.
- **FR-004**: Feature 001 artifacts MUST not contradict current shipped
  behavior for report section count, parser diagnostics, Go version, or
  constitution principle count.
- **FR-005**: Validation MUST include Go tests, constitution guard, Spec Kit
  prerequisite/analyze checks while active, canonical-path searches, and PR
  review. Full-repository health validation SHOULD also run Worker typecheck
  and tests even when this branch does not change Worker files.
- **FR-006**: No governed first-party source file may exceed 1000 lines.

### Canonical Path Expectations *(include when changing existing behavior)*

- **Canonical owner/path**: `.specify/scripts/bash/common.sh` for Bash Spec Kit
  helper functions; PowerShell git extension helper code only for PowerShell
  execution; `parse` netstat timestamp handling for netstat diagnostics;
  current feature artifacts for audit evidence.
- **Old path treatment**: remove duplicate Bash helper fallback loading and
  avoid hidden absent-path probes; update stale feature 001 wording rather than
  adding parallel interpretations.
- **External degradation**: no new external degradation path is introduced.
- **Review check**: reviewers verify no old fallback helper path remains and
  malformed netstat `TS` headers are covered by tests.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test -count=1 ./...` passes.
- **SC-002**: `make lint` passes.
- **SC-003**: Worker typecheck and tests pass.
- **SC-004**: Constitution guard passes for the branch diff.
- **SC-005**: Canonical-path scans show no live references to retired Spec Kit
  helper fallback paths.
- **SC-006**: All PR review findings are fixed or dismissed with rationale
  before merge.

## Assumptions

- This round fixes confirmed audit findings only; it does not introduce new
  product behavior.
- Feature 001 remains historical, but its normative-looking text should not
  contradict the current repository state.
