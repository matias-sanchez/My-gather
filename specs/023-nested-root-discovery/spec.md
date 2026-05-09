# Feature Specification: Nested Root Discovery for Directory Inputs

**Feature Branch**: `023-nested-root-discovery`
**Created**: 2026-05-08
**Status**: Draft
**Input**: User description: "Nested pt-stalk root discovery for directory inputs. Make `my-gather <dir>` succeed when the actual pt-stalk capture lives in a subdirectory of the directory the user pointed at, mirroring what archive inputs already do."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Find a single nested capture under a case folder (Priority: P1)

A support engineer downloads a case attachment, extracts it, and points
`my-gather` at the case folder. The actual pt-stalk capture lives several
subdirectories below (for example `case/host/tmp/pt/collected/host/`). Today
the tool exits with `is not recognised as a pt-stalk output directory` and the
engineer has to hunt for the real path. After this feature, the tool finds the
capture and produces a report.

**Why this priority**: This is the dominant real-world layout. An empirical
survey of the engineer's local capture corpus (57 distinct pt-stalk roots)
shows roughly 68% live 6+ subdirectory levels below the case folder, and the
remainder still typically live one or two levels below. The feature is the
single highest-impact UX fix for the directory-input path.

**Independent Test**: Stand up a synthetic directory tree that contains a
single mini pt-stalk capture nested 6 levels deep under the input root. Run
`my-gather <input-root>`. The tool succeeds, produces a report, and the
generated HTML names the discovered host.

**Acceptance Scenarios**:

1. **Given** a directory whose only pt-stalk capture lives 6 levels below the
   input path, **When** the engineer runs `my-gather <input>`, **Then** the
   tool discovers the nested capture, generates the report, and writes it to
   the requested output path.
2. **Given** a directory that already IS the pt-stalk capture root (the
   existing supported layout), **When** the engineer runs `my-gather
   <input>`, **Then** the tool generates the report exactly as it does today
   with no behavior change.
3. **Given** a directory containing the pt-stalk capture one level below
   (e.g. `case/host/`), **When** the engineer runs `my-gather <input>`,
   **Then** the tool discovers and renders that capture.

---

### User Story 2 - Refuse and clearly explain when several captures coexist (Priority: P1)

A support engineer points `my-gather` at a folder that holds multiple
distinct pt-stalk captures (for example, three host folders side-by-side in a
multi-host case). The tool must refuse to silently pick one - silent
selection would risk rendering the wrong host's data - and must tell the
engineer exactly which paths it found so they can re-invoke with the correct
one.

**Why this priority**: Multi-host cases are common (~14% of the local
corpus). Silently picking the lexically-first one would produce a misleading
report under load. The same constraint already governs archive input today;
extending it to directory input is required for correctness, not just
ergonomics.

**Independent Test**: Stand up a synthetic directory tree containing two
pt-stalk roots in distinct subdirectories. Run `my-gather <input>`. The tool
exits non-zero, produces no output file, and the stderr message names both
discovered roots in lexical order.

**Acceptance Scenarios**:

1. **Given** an input directory containing two pt-stalk capture roots in
   distinct subdirectories, **When** the engineer runs `my-gather <input>`,
   **Then** the tool exits non-zero, writes no report file, and stderr lists
   both roots in lexical order with a one-line instruction telling the
   engineer to re-invoke pointing at one of them.
2. **Given** an input directory containing three pt-stalk capture roots,
   **When** the engineer runs `my-gather <input>`, **Then** all three are
   listed in stderr in lexical order.
3. **Given** the same multi-root input run twice in succession, **When** the
   stderr output of both runs is compared, **Then** the lines are
   byte-identical (deterministic enumeration order).

---

### User Story 3 - Refuse and clearly explain when no capture exists (Priority: P1)

A support engineer accidentally points `my-gather` at a directory that
contains no pt-stalk capture at all (the wrong folder, or a folder of
unrelated files). The tool must clearly tell them no capture was found AND
indicate that subdirectories were searched, so the engineer does not waste
time wondering whether a typo in the path or a missing top-level signal is
the cause.

**Why this priority**: This is the existing failure mode users hit today.
The feature must keep the failure clear and actionable, distinct from the
multiple-roots case, and must explicitly signal that subdirectories were
searched (so the engineer knows the answer is "the capture isn't in this
tree at all," not "the tool only looks at the top level").

**Acceptance Scenarios**:

1. **Given** an input directory that contains no pt-stalk capture at any
   nesting level within the searched depth, **When** the engineer runs
   `my-gather <input>`, **Then** the tool exits non-zero, writes no report,
   and stderr clearly states the directory and its subdirectories were
   searched and no pt-stalk collection was found.
2. **Given** an input path that does not exist, **When** the engineer runs
   `my-gather <input>`, **Then** the existing path-not-found error is
   returned unchanged (this is a structural error, not a discovery error).

---

### Edge Cases

- **Hidden directories**: A pt-stalk capture nested only inside a hidden
  directory (for example `.cache/pt/...`) is treated as if no capture is
  present. Hidden directories (those whose name begins with `.`) MUST be
  skipped during search to keep the walk fast on real-world trees that
  contain `.git`, `.cache`, `.DS_Store`-bearing folders, etc.
- **Excessive nesting**: A pt-stalk capture nested more deeply than the
  search-depth limit is treated as if no capture is present. The engineer
  is told the directory and its subdirectories were searched; they can
  re-invoke pointing at the actual root.
- **Symlinks inside the input tree**: Symlinks MUST NOT be followed during
  search. This prevents cycles and prevents the tool from escaping the
  user's intended input tree (Principle II). A symlinked pt-stalk capture
  at the top level still works because the user pointed the tool there
  directly.
- **Permission errors deep in the tree**: A subdirectory the user cannot
  read MUST NOT abort the whole search. The tool MUST continue searching
  the rest of the tree and produce its normal "found N roots" outcome
  based on what it could read. (Principle III: graceful degradation.)
- **Many shallow directories**: An input directory containing thousands of
  unrelated subdirectories (e.g., a downloads folder) MUST not cause the
  search to take more than a few seconds. The depth cap and a total
  scanned-entry cap together bound the worst case.
- **Top-level recognition + ambiguity check**: When the input
  directory IS itself a pt-stalk root, the walker still descends into
  its subtree to check for additional pt-stalk roots that would make
  the input ambiguous. The single-root case (input is a clean
  pt-stalk root with no nested pt-stalk-shaped subtrees) returns
  exactly the input's absolute path. The ambiguous case (input is a
  pt-stalk root AND a nested subtree is also a pt-stalk root) returns
  the multi-root error.
- **Archive-extracted layout**: Archive inputs already discover nested
  roots via the same logic. After this feature, both directory and
  archive inputs MUST converge on a single shared discovery
  implementation; they MUST NOT have parallel rule sets.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST accept a directory input that contains the
  pt-stalk capture in a subdirectory and produce a report from that
  subdirectory's contents.
- **FR-002**: When the input directory itself is recognized as a pt-stalk
  root, the tool MUST report it as the parse root unless additional
  pt-stalk roots are found in the bounded subtree below; if any are
  found, the multi-root error fires (FR-005). The walker MUST descend
  into the recognised root's subtree to make this ambiguity check
  honest, mirroring the pre-feature `findExtractedPtStalkRoot`
  behaviour for archive inputs.
- **FR-003**: When the input directory is not itself a pt-stalk root, the
  tool MUST search its subdirectories for pt-stalk roots, using the same
  recognition rule used today for the top-level fast path and for archive
  inputs.
- **FR-004**: When exactly one pt-stalk root is found in the searched tree,
  the tool MUST proceed to parse and render that root.
- **FR-005**: When more than one pt-stalk root is found, the tool MUST
  exit non-zero, write no report file, and write a single stderr message
  that names the discovered roots in lexical order along with an
  instruction to re-invoke the tool pointing at one of them.
- **FR-006**: When no pt-stalk root is found in the searched tree, the
  tool MUST exit non-zero, write no report file, and write a stderr
  message that explicitly conveys both the directory was searched and
  that subdirectories were searched too (so the engineer knows the
  answer is not "top level only").
- **FR-007**: The search MUST be depth-bounded. Subdirectories at depths
  beyond the bound MUST NOT be searched. The bound MUST be set so that
  every real-world layout in the engineer's local capture corpus
  (deepest observed: 6 directories below the input root) succeeds, with
  modest headroom.
- **FR-008**: The search MUST skip hidden subdirectories (those whose
  name begins with `.`).
- **FR-009**: The search MUST NOT follow symbolic links to other
  directories. The input directory's own type (regular dir vs symlink at
  the root) is unchanged from current behavior.
- **FR-010**: The search MUST be tolerant of permission errors on
  individual subdirectories: an unreadable subdirectory does not abort
  the search; the tool continues with the rest of the tree.
- **FR-011**: The search ordering MUST be deterministic so that two
  invocations on the same input produce byte-identical stderr output
  in the multi-root and zero-root cases (Principle IV).
- **FR-012**: The discovery implementation used by directory input MUST
  be the same implementation used by archive input. There MUST be one
  canonical "find the pt-stalk root inside this directory tree" code
  path, not two (Principle XIII).
- **FR-013**: The tool MUST NOT write, create, rename, or delete any
  file or directory anywhere within the input tree during discovery
  (Principle II), regardless of how deep the search descends.
- **FR-014**: The CLI contract MUST be updated to document the new
  behavior: a directory input is interpreted as either a pt-stalk root
  or a directory containing one, with subdirectories searched up to the
  defined depth bound.
- **FR-015**: The CLI `--help` output MUST be updated to reflect that
  the input directory may contain the pt-stalk capture in a
  subdirectory, so the user-facing help text matches behavior.

### Canonical Path Expectations

- **Canonical owner/path**: The one place that, given a directory tree,
  decides where the pt-stalk root is. Today this lives in
  `cmd/my-gather/archive_input.go` as `findExtractedPtStalkRoot` and is
  used only by the archive path. After this feature, the same function
  (renamed and relocated as appropriate, see plan) is the canonical
  path used by both directory and archive inputs.
- **Old path treatment**: The directory-input branch in
  `cmd/my-gather/archive_input.go::prepareInput` that returns the input
  path as-is when it is a directory MUST be updated in the same change
  to call the canonical discovery function. The archive-input call site
  MUST switch to the canonical function. No second copy of the
  discovery rule may remain anywhere in the codebase. The deliberate
  "subdirectories are not walked in v1" comment in `parse/parse.go` MUST
  be updated to point at the new canonical function so future readers
  do not assume the old constraint still holds.
- **External degradation**: The "no roots found" and "multiple roots
  found" outcomes are external degradation paths (Principle III): the
  tool reports them as typed errors with structured messages and exits
  non-zero rather than guessing. Both outcomes are observable via stderr
  and exit code, and both are covered by tests.
- **Review check**: Reviewers verify on the diff that (a) `parse.go`'s
  top-level `Discover` is still single-root and read-only, (b) the
  canonical discovery function is called from both directory and
  archive paths, (c) no parallel walk-and-detect helper has been added
  elsewhere, and (d) the CLI contract and `--help` text reflect the
  new behavior.

### Key Entities

- **Pt-stalk root**: A directory that contains at least one pt-stalk
  timestamped snapshot file (e.g., `YYYY_MM_DD_HH_MM_SS-<collector>`)
  or a recognized summary file (`pt-summary.out`,
  `pt-mysql-summary.out`). The recognition rule is the same one used
  today; this feature does not redefine what counts as a root.
- **Input tree**: The directory the user passed on the command line,
  considered together with its subdirectories down to the depth bound,
  excluding hidden subdirectories and excluding directories reached
  only via symlink.
- **Discovery outcome**: One of (a) exactly one pt-stalk root found
  (the report is generated against that root), (b) more than one root
  found (the tool exits with the multiple-roots error and lists them),
  (c) zero roots found (the tool exits with the no-root error and
  states subdirectories were searched).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For every directory in the engineer's local capture
  corpus (`~/Documents/Incidents`) where exactly one pt-stalk root
  exists somewhere in the tree, `my-gather <dir>` succeeds and
  generates a report. Spot-checked on at least one case from each
  observed nesting depth (2, 3, 4, 5, 6, and 7 levels below the case
  folder). At least 95% of cases that have exactly one root in their
  case folder render successfully when invoked at the case folder.
- **SC-002**: For every directory in the corpus that contains more
  than one pt-stalk root, `my-gather <dir>` exits non-zero, writes no
  report, and stderr names every discovered root in lexical order.
  Spot-checked on at least one multi-host case from the corpus.
- **SC-003**: For an existing valid invocation (the user already
  pointed at the pt-stalk root), the tool's behavior is bit-identical
  to today: same exit code, same stderr lines, same generated report
  bytes (Principle IV regression check on golden fixtures).
- **SC-004**: The added discovery walk completes in well under one
  second on every directory in the engineer's local capture corpus,
  bounded by `MaxEntries = 100000`. This is enforced structurally by
  the entry cap rather than by a wall-clock test (CI runners are
  shared and a wall-clock cap would be flaky); the corpus spot check
  in the quickstart is the operator-facing demonstration.
- **SC-005**: The shared canonical discovery function is referenced
  from both the directory-input and archive-input call sites, and a
  static check (or reviewer audit) confirms there is no second
  walk-and-detect helper in the codebase.
- **SC-006**: The CLI contract document and `--help` text both
  describe the new behavior and match the implementation. A
  contract-level test or quickstart command demonstrates a directory
  input with a nested capture rendering successfully.

## Assumptions

- The recognition rule for "this directory looks like a pt-stalk root"
  (top-level timestamped snapshot file or recognized summary file) is
  correct as-is. This feature reuses that rule and does not change
  what counts as a root.
- A depth bound of 8 levels below the input root is sufficient. The
  deepest observed real-world layout in the local corpus puts the
  pt-stalk root 6 directories below the case folder; an 8-level bound
  gives 2 levels of headroom while still preventing pathological
  walks. If a deeper layout appears in the wild, the bound can be
  raised in a follow-up amendment without re-architecting the feature.
- A scanned-entry cap (paired with the depth cap) is acceptable as a
  belt-and-braces guard against pathological trees. The cap is set
  high enough that no real-world capture corpus reaches it; if it is
  ever exceeded, the discovery exits with the "no root found" outcome
  and tells the user the search was bounded.
- The archive-input behavior is the reference. After this feature,
  directory and archive inputs share the same discovery function and
  the same error types. Renaming the existing
  `multiplePtStalkRootsError` to make it generic across both inputs
  is acceptable; preserving the name as-is is also acceptable.
- The user's "Commit + push after implementing" memory applies: the
  feature ships on its own branch with a PR, not as a local-only
  change. The branch was created at the start of the pipeline by the
  `before_specify` hook.
- This feature does not change parse, model, or render behavior once
  the root is selected. It only changes which directory is selected
  as the parse root.
