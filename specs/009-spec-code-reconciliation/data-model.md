# Data Model: Spec And Code Reconciliation

## Reconciliation Finding

A concrete inconsistency between checked-in specs, task files, active pointers,
or code coverage.

Fields:

- `id`: stable analysis identifier
- `category`: workflow drift, status drift, task tracking drift, coverage gap,
  or stale placeholder
- `affected_paths`: checked-in files that need reconciliation
- `resolution`: documentation update, test addition, or status update

Rules:

- Findings must resolve without weakening the constitution.
- Historical context may be preserved when clearly labelled.
- Code behavior changes are avoided unless a documented contract currently
  fails.

## Active Feature Pointer

The durable repository state that tells agents which feature is current.

Fields:

- `feature_directory`: `.specify/feature.json` value
- `agent_context`: active block in `AGENTS.md`
- `claude_context`: active block in `CLAUDE.md`

Rules:

- All three pointers must name the same feature while a feature branch is
  active.
- On `main`, the pointer may name the latest shipped feature only when the
  agent context files say there is no active feature.

## Archive Error Test

A focused CLI test that validates one documented input failure mode.

Fields:

- `fixture`: temporary archive or regular file
- `expected_exit_code`: documented CLI exit category
- `expected_message`: stable stderr substring proving clear user feedback

Rules:

- Fixtures are created under `t.TempDir`.
- Tests must not depend on local incident data.
- Tests must not write inside source input trees.
