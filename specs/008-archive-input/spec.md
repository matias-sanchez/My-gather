# Feature Specification: Archive Input

**Feature Branch**: `008-archive-input`  
**Created**: 2026-04-29  
**Status**: Complete
**Input**: User description: "Search `/Users/matias/Documents/Incidents/` for folders containing pt-stalk data and compressed files, then adapt the tool so it can accept either a folder or compressed file. Compressed files must extract into a temporary folder and open through the tool as well."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run Reports Directly From Archives (Priority: P1)

A support engineer receives a compressed pt-stalk capture and wants to generate
the same HTML report without manually extracting the archive first.

**Why this priority**: Many incident attachments arrive as `.zip`, `.tar`,
`.tar.gz`, `.tgz`, or `.gz` files. Requiring manual extraction slows triage and
creates inconsistent local folder layouts.

**Independent Test**: Run `my-gather --overwrite -o /tmp/report.html
capture.zip` and `my-gather --overwrite -o /tmp/report.html capture.tar.gz`
against archives that contain a nested pt-stalk directory. Each command exits
0 and writes a non-empty self-contained report.

**Acceptance Scenarios**:

1. **Given** a supported archive that contains exactly one pt-stalk collection,
   **When** the user passes the archive as the positional input, **Then** the
   tool extracts it to a temporary directory, discovers the pt-stalk root, and
   renders the report.
2. **Given** the same collection as an extracted directory or as an archive,
   **When** the user generates a report, **Then** the report uses the same
   parser, model, renderer, and Advisor paths.
3. **Given** archive processing completes or fails, **When** the command exits,
   **Then** temporary extraction files are removed and the original archive is
   not modified.

---

### User Story 2 - Preserve Directory Input Compatibility (Priority: P1)

A support engineer already has an extracted pt-stalk directory and expects the
existing command and exit codes to continue working.

**Why this priority**: Directory input is the current primary workflow and must
remain stable.

**Independent Test**: Run the existing directory quickstart command against
`testdata/example2` and a real incident pt-stalk directory. Both commands keep
their prior behavior.

**Acceptance Scenarios**:

1. **Given** an existing pt-stalk directory, **When** it is passed as input,
   **Then** no archive extraction occurs and the existing parse flow runs.
2. **Given** `--out` points inside a directory input tree, **When** the command
   starts, **Then** the existing output-inside-input guard still rejects the
   run with exit code 7.
3. **Given** no input or more than one input is passed, **When** flags are
   parsed, **Then** usage errors still use exit code 2.

---

### User Story 3 - Reject Unsafe Or Ambiguous Archives (Priority: P2)

A support engineer needs archive handling to be safe under untrusted customer
attachments and clear when an archive is not a single report input.

**Why this priority**: Archives can contain path traversal entries, symlinks,
multiple host captures, or unrelated compressed logs. The tool must not write
outside its temporary workspace or silently choose the wrong collection.

**Independent Test**: Run focused tests with a traversal zip entry, an
unsupported file type, an archive with no pt-stalk root, and an archive with
multiple pt-stalk roots. Each case exits with the documented non-zero code and
does not create unexpected files.

**Acceptance Scenarios**:

1. **Given** an archive contains `../` or absolute extraction paths, **When**
   the tool processes it, **Then** extraction is rejected before any outside
   path can be written.
2. **Given** an archive contains multiple pt-stalk roots, **When** the tool
   processes it, **Then** the command fails clearly instead of selecting one
   silently.
3. **Given** a regular file with an unsupported extension is passed, **When**
   the command starts, **Then** it exits with an input-path error naming the
   supported archive formats.

### Edge Cases

- Archives may wrap the pt-stalk root in one or more parent directories.
- `.gz` files may contain a single decompressed file or a tar stream with a
  non-standard filename; tar streams are detected by content.
- Archives may contain hidden metadata files or directories that are unrelated
  to pt-stalk data.
- Archives may contain symlinks, hardlinks, devices, or other non-regular
  entries; these are rejected.
- Archives may expand beyond the supported total input size and must fail
  before unbounded extraction.
- Incident trees may contain many unrelated compressed logs; names alone are
  not sufficient to identify pt-stalk archives.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST accept one positional `<input>` that can be either
  a pt-stalk directory or a supported archive file.
- **FR-002**: Supported archive formats MUST include `.zip`, `.tar`,
  `.tar.gz`, `.tgz`, and `.gz`.
- **FR-003**: Directory input MUST continue using the existing
  `parse.Discover` code path without temporary extraction.
- **FR-004**: Archive input MUST extract only into a newly-created temporary
  directory outside the input tree.
- **FR-005**: Archive extraction MUST remove the temporary directory on both
  success and failure.
- **FR-006**: Archive extraction MUST reject absolute paths, parent-directory
  traversal, symlinks, hardlinks, devices, and other non-regular entries.
- **FR-007**: Archive extraction MUST enforce the existing 1 GB total
  collection bound while writing extracted content.
- **FR-008**: After archive extraction, the tool MUST discover exactly one
  pt-stalk root by using the same top-level recognition signals as
  `parse.Discover`: timestamped collector files or summary files.
- **FR-009**: If an archive contains no pt-stalk root, the CLI MUST fail with
  exit code 4.
- **FR-010**: If an archive contains multiple pt-stalk roots, the CLI MUST
  fail clearly instead of selecting one silently.
- **FR-011**: Unsupported regular input files MUST fail with exit code 3 and
  list the supported archive formats.
- **FR-012**: The rendered report for archive input MUST be produced through
  the existing parser, model, findings, and renderer pipeline.
- **FR-013**: Archive handling MUST add no runtime network access and no new
  third-party dependencies.
- **FR-014**: Help text and CLI contracts MUST describe `<input>` as directory
  or supported archive, replacing directory-only wording.

### Key Entities

- **Input Path**: The user-provided positional argument. It resolves to either
  a directory or a regular archive file.
- **Temporary Extraction Directory**: A private workspace created for one
  archive run and removed before process exit.
- **Extracted pt-stalk Root**: The single extracted directory that contains
  timestamped pt-stalk collector files or recognized summary files.
- **Incident Inventory**: Local scan evidence from
  `/Users/matias/Documents/Incidents/` used to validate that both directory
  and archive workflows are common.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The local incident inventory identifies parse-compatible
  pt-stalk candidate directories and archive files under the incident tree.
- **SC-002**: `go test ./cmd/my-gather ./parse` covers directory recognition,
  zip archive input, tar-gzip archive input, and unsafe archive rejection.
- **SC-003**: A report can be generated from an archive containing a nested
  pt-stalk directory without manual extraction.
- **SC-004**: Unsafe archive entries do not create files outside the temporary
  extraction directory.
- **SC-005**: Existing directory-input behavior remains compatible with the
  current quickstart commands and exit codes.

## Assumptions

- A single `my-gather` invocation renders one pt-stalk collection. Multi-host
  archives remain out of scope until the product supports multi-report or
  collection selection.
- Standard-library archive support is preferred over new dependencies.
- The incident inventory is local evidence and is not committed verbatim
  because customer paths can contain sensitive identifiers.
