# Feature Specification: Full Compliance Fixes

**Feature Branch**: `013-full-compliance-fixes`  
**Created**: 2026-05-05  
**Status**: Complete
**Input**: User description: "fix em all, reach 100% all"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintainer Sees No Confirmed Compliance Findings (Priority: P1)

A maintainer runs the post-merge repository audit and receives a clean result
for the confirmed findings from the May 5, 2026 multi-agent validation.

**Why this priority**: The request is explicitly to close every confirmed
constitution, spec, source, test, and documentation finding rather than leave a
follow-up backlog.

**Independent Test**: Run the full validation commands listed in `quickstart.md`
and verify there are zero confirmed findings in the updated audit contract.

**Acceptance Scenarios**:

1. **Given** the repository is on this feature branch, **When** the validation
   suite is run, **Then** all mechanical checks pass.
2. **Given** the repository artifacts are reviewed against the latest
   constitution, **When** confirmed audit findings are checked, **Then** each is
   either fixed or explicitly classified as an accepted non-violation.

---

### User Story 2 - Spec Kit Uses One Canonical Feature Path (Priority: P1)

A maintainer runs Spec Kit workflows and the repository resolves the feature
directory only through the canonical pointer or an explicit override.

**Why this priority**: The strongest remaining Principle XIII concern was a
fallback-style feature-directory path and a retained old feature-creation
script.

**Independent Test**: Run Spec Kit prerequisite checks on the feature branch and
verify only the extension git feature hook owns branch creation.

**Acceptance Scenarios**:

1. **Given** `.specify/feature.json` points to this feature, **When** a Spec Kit
   prerequisite check runs, **Then** it resolves this feature directory.
2. **Given** `.specify/feature.json` is missing or invalid, **When** a Spec Kit
   workflow needs the active feature, **Then** it fails clearly instead of
   silently deriving a feature from the branch name.

---

### User Story 3 - Parser and Helper Paths Are Canonical (Priority: P2)

A maintainer reviews parser and render helper code and finds no quiet wrapper,
silent timestamp fallback, or duplicate number-formatting helper in the
confirmed locations.

**Why this priority**: These were the remaining source-level Principle XIII and
Principle III concerns.

**Independent Test**: Run the focused parser, render, and reportutil tests plus
the canonical-path search checks in `quickstart.md`.

**Acceptance Scenarios**:

1. **Given** malformed `TS` epochs in netstat inputs, **When** parsing runs,
   **Then** the snapshot-start substitution emits a structured diagnostic.
2. **Given** environment meminfo sidecar parsing, **When** the renderer needs
   meminfo data, **Then** it calls the diagnostic parser path rather than a
   quiet wrapper.
3. **Given** InnoDB render values need comma formatting, **When** render code
   formats them, **Then** the formatting goes through `reportutil`.

### Edge Cases

- Historical feature specifications may remain intentionally historical, but
  they must not contradict the current shipped behavior in normative wording.
- A completed feature on `main` should not be advertised as active for future
  `/speckit-*` workflows.
- External feedback degradation remains allowed only where it is visible and
  documented as external degradation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The repository MUST contain a new feature artifact set for the
  compliance-fix pass.
- **FR-002**: `AGENTS.md`, `CLAUDE.md`, and `.specify/feature.json` MUST stay
  aligned while the feature is active and MUST document a no-active-feature
  state suitable for `main` after completion.
- **FR-003**: Spec Kit feature-directory resolution MUST use the canonical
  `.specify/feature.json` pointer or an explicit `SPECIFY_FEATURE_DIRECTORY`
  override, with no branch-name fallback path.
- **FR-004**: Only the git extension feature hook MUST own feature branch
  creation; the old core create-new-feature script MUST be removed from the
  maintained source path.
- **FR-005**: Environment meminfo parsing MUST expose one canonical diagnostic
  parser path to first-party callers.
- **FR-006**: Netstat timestamp degradation MUST emit structured diagnostics
  when a malformed `TS` epoch falls back to the snapshot timestamp.
- **FR-007**: Shared report integer formatting MUST be owned by `reportutil`,
  with no duplicate InnoDB formatting helper.
- **FR-008**: Mechanical source-size enforcement MUST cover the maintained
  source extensions named by Principle XV, including PowerShell scripts and
  HTML implementation files.
- **FR-009**: Parser fixture/golden enforcement text and tests MUST align with
  the constitution's mechanical gate wording.
- **FR-010**: Worker test configuration MUST use the Workers-compatible Vitest
  pool promised by feature 003 documentation.
- **FR-011**: Historical spec drift called out by the audit MUST be corrected
  without changing runtime behavior.
- **FR-012**: Validation MUST include Go tests, Worker tests, constitution
  guard, Spec Kit prerequisite/analyze checks while active, and post-fix
  canonical-path searches.

### Canonical Path Expectations *(include when changing existing behavior)*

- **Canonical owner/path**: `.specify/feature.json` for active feature
  resolution; `.specify/extensions/git/scripts/*/create-new-feature.*` for
  branch creation; `ParseEnvMeminfoWithDiagnostics` for env meminfo parsing;
  `reportutil` for shared report formatting.
- **Old path treatment**: old core feature-creation script deleted; branch-name
  feature-directory fallback removed; quiet env meminfo wrapper removed;
  local InnoDB comma formatter removed.
- **External degradation**: feedback Worker failure remains visible external
  degradation and is documented as accepted.
- **Review check**: reviewers verify the old paths are absent and no new
  compatibility shim or silent fallback replaces them.

### Key Entities *(include if feature involves data)*

- **Compliance finding**: a confirmed audit item with severity, affected paths,
  fix status, and validation evidence.
- **Canonical feature pointer**: `.specify/feature.json` field naming the active
  or latest shipped feature directory.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Full Go validation passes with `go test -count=1 ./...`.
- **SC-002**: Worker validation passes with `npm run typecheck` and `npm test`.
- **SC-003**: Constitution guard passes in CI mode for this branch diff.
- **SC-004**: Spec Kit prerequisite and analysis checks pass while this branch
  advertises the active feature.
- **SC-005**: Canonical-path searches show no retained confirmed old paths from
  the May 5 audit.
- **SC-006**: No governed first-party source file exceeds 1000 lines.

## Assumptions

- The goal is to fix confirmed audit findings from the May 5, 2026 validation,
  not to add new product features.
- Historical feature 001 can remain an in-progress spec, but its normative text
  should not contradict the current shipped report shape.
- No constitution amendment is needed because the changes tighten alignment with
  existing principles.
