# Feature Specification: Compliance Closure

**Feature Branch**: `015-compliance-closure`
**Created**: 2026-05-05
**Status**: Ready for review
**Input**: User description: "create a new branch and add all of the findings and fix everything into that one; complete everything to 100% ready"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintainer Sees Audit Findings Closed (Priority: P1)

A maintainer reviews the latest repository audit and sees every confirmed
spec, source, tooling, and enforcement finding either fixed or explicitly
classified as non-blocking evidence.

**Why this priority**: The previous audit showed green tests but partial
constitution/spec alignment due to historical spec drift, render behavior
gaps, extension-state drift, and weak route-level coverage.

**Independent Test**: Run the validation commands in `quickstart.md` and
verify `contracts/audit.md` maps every confirmed finding to a completed fix.

**Acceptance Scenarios**:

1. **Given** historical specs 001, 002, and 012 are reviewed, **When** their
   status and superseded behavior text is read, **Then** it does not contradict
   shipped behavior or current constitution rules.
2. **Given** a known collector file has an unsupported pt-stalk version,
   **When** the report renders, **Then** the section shows an unsupported-version
   state naming the source rather than a generic missing/unparseable state.
3. **Given** repository enforcement metadata is reviewed, **When** extension,
   agent, and skill pointers are inspected, **Then** they identify the same
   canonical paths and installed extension state.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The branch MUST include a feature artifact set for the compliance
  closure round.
- **FR-002**: Historical shipped specs MUST use shipped/historical statuses and
  MUST NOT present known backlog as current unplanned implementation work.
- **FR-003**: Feature 001 deterministic-output wording MUST align with the
  current strict byte-identical report generation model.
- **FR-004**: Feature 002 plan/tasks MUST mark the old GitHub Discussions flow
  as superseded by Feature 003 and MUST use the current 10-minute voice cap.
- **FR-005**: Feature 012 MUST be marked complete.
- **FR-006**: Supported collector files with unsupported format versions MUST
  render an unsupported-version state naming the file.
- **FR-007**: InnoDB scalar callout formatting MUST use the canonical
  `reportutil` helpers.
- **FR-008**: Spec Kit extension metadata MUST match the enabled git extension.
- **FR-009**: Agent/skill guidance MUST use canonical feature pointers and live
  command-doc paths.
- **FR-010**: Tests/enforcement SHOULD cover the fixed source behavior and
  high-signal Worker route gaps without adding new product behavior.
- **FR-011**: No governed first-party source file may exceed 1000 lines.

### Canonical Path Expectations *(include when changing existing behavior)*

- **Canonical owner/path**: `parse` owns source-file parse status;
  `render` owns report state presentation; `reportutil` owns reusable integer
  formatting; `.specify/feature.json` owns machine-readable feature selection;
  `.specify/extensions.yml` owns installed extension metadata.
- **Old path treatment**: historical text is updated or explicitly marked
  superseded; no compatibility shims, hidden fallbacks, or duplicate helper
  paths are introduced.
- **External degradation**: unsupported pt-stalk versions are observable to the
  reader as unsupported-version states.
- **Review check**: reviewers verify each audit contract row is fixed with
  tests or explicit documentation evidence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test -count=1 ./...` passes.
- **SC-002**: `make lint` passes.
- **SC-003**: Worker typecheck and tests pass.
- **SC-004**: Constitution guard passes.
- **SC-005**: Source-size scan finds no governed source file over 1000 lines.
- **SC-006**: A repeated audit finds no confirmed high or medium findings from
  the previous audit list.

## Assumptions

- This pass closes confirmed audit findings and targeted coverage gaps; it does
  not add user-facing features beyond making already-specified behavior work.
- Historical features may remain historical with documented backlog, provided
  they do not contradict current shipped behavior.
