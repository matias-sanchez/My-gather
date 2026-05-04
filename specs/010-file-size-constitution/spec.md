# Feature Specification: File Size Constitution Guard

**Feature Branch**: `010-file-size-constitution`
**Created**: 2026-05-01
**Status**: Complete
**Input**: User description: "Add a constitution rule against source-code god files and source code files over 1000 lines, create a new spec for it, and ensure the whole repository aligns with the rule."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Prevent God Files At Review Time (Priority: P1)

A maintainer reviews a pull request and needs a mechanical guard that rejects
large first-party source code files before they become expensive to understand
and refactor.

**Why this priority**: The project already has mechanical constitution gates.
File-size limits should be enforced by CI rather than remembered manually in
review.

**Independent Test**: Run the coverage test package. It fails if a governed
first-party file has more than 1000 lines and reports the offending path.

**Acceptance Scenarios**:

1. **Given** a first-party source code file exceeds 1000
   lines, **When** the coverage tests run, **Then** the test fails with the path
   and line count.
2. **Given** raw fixtures, reference captures, specs, docs, JSON data,
   generated lockfiles, or golden snapshots exceed 1000 lines, **When** the
   coverage tests run, **Then** they are outside this source-code rule.

---

### User Story 2 - Current Repository Complies (Priority: P1)

A maintainer wants to merge the new rule only when the current repository
already passes it.

**Why this priority**: A constitution rule that fails on main would block all
future work and create noise instead of quality.

**Independent Test**: Run the file-size test and `go test ./...` on this branch.
Both pass with no governed source code file over 1000 lines.

**Acceptance Scenarios**:

1. **Given** `render/assets/app.js` is currently over 1000 lines, **When** this
   feature lands, **Then** its behavior is preserved through smaller ordered
   embedded asset files.
2. **Given** `render/assets/app.css` is currently over 1000 lines, **When** this
   feature lands, **Then** its styling is preserved through smaller ordered
   embedded stylesheet files.
---

### User Story 3 - Constitution Documents The Boundary (Priority: P2)

A future contributor needs to know which source code files the 1000-line rule
governs and which large non-source artifacts are outside the rule.

**Why this priority**: The repository legitimately contains large pt-stalk
fixtures. Without a clear boundary, the rule would conflict with the
constitution's fixture and raw-reference principles.

**Independent Test**: Read the constitution and the coverage test. Both describe
the same governed scope and exemptions.

**Acceptance Scenarios**:

1. **Given** a large raw fixture under `testdata/` or `_references/`, **When**
   the rule is applied, **Then** it is outside the source-code limit.
2. **Given** a large maintained Go, JavaScript, TypeScript, CSS, shell, or
   template source file, **When** the rule is applied, **Then** it is governed by
   the 1000-line limit.

### Edge Cases

- Some generated third-party source artifacts, such as vendored minified
  libraries, can be large by byte count without being god files. These must be
  exempted by vendor-style directory class or explicit reviewed path allowlist;
  maintained first-party minified source remains governed.
- Raw fixtures and golden snapshots must remain faithful to source data and
  should not be reformatted just to satisfy a maintainability rule.
- Splitting embedded JS/CSS must preserve deterministic rendered output and
  offline report behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The constitution MUST include a core principle forbidding governed
  first-party source code files over 1000 lines.
- **FR-002**: The constitution MUST document that non-source artifacts such as
  raw fixtures, reference captures, specs, docs, JSON data, generated lockfiles,
  and golden snapshots are outside this source-code rule.
- **FR-003**: A mechanical Go test MUST enforce the governed source file line
  limit.
- **FR-004**: The mechanical test MUST fail with a clear path and line count for
  every governed violation.
- **FR-005**: `render/assets/app.js` MUST be split into ordered embedded files,
  each at or below 1000 lines.
- **FR-006**: `render/assets/app.css` MUST be split into ordered embedded files,
  each at or below 1000 lines.
- **FR-007**: Report rendering MUST continue to embed one self-contained CSS
  block and one self-contained app JS block in deterministic order.
- **FR-008**: Validation MUST include the file-size coverage test and the full Go
  suite before opening the PR.

### Key Entities

- **Governed Source File**: A checked-in first-party source code file subject to
  the 1000-line limit.
- **Out-Of-Scope Artifact**: A checked-in non-source artifact such as a spec,
  doc, JSON data file, raw fixture, reference capture, golden snapshot, or
  generated lockfile.
- **Ordered Embedded Asset Part**: A JS or CSS asset fragment whose filename
  determines concatenation order.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test ./tests/coverage -run TestGovernedSourceFileLineLimit`
  passes.
- **SC-002**: `go test -count=1 ./...` passes.
- **SC-003**: No governed checked-in source code file has more than 1000 lines.
- **SC-004**: Rendered CSS and JS are concatenated from smaller files in stable
  lexical order.

## Assumptions

- The 1000-line limit is a maintainability rule for governed first-party source
  code files, not raw evidence captured from pt-stalk, specs, docs, JSON data,
  or committed golden snapshots.
- Line count is the first mechanical gate; deeper complexity metrics can be
  added later if line count alone proves insufficient.
