# Data Model: File Size Constitution Guard

## Governed Source File

A checked-in first-party source code file subject to the 1000-line maximum.

Fields:

- `path`: repository-relative path
- `line_count`: number of newline-delimited lines
- `classification`: Go, JavaScript, TypeScript, CSS, shell, template, or other
  source-code extension

Rules:

- Governed source files must not exceed 1000 lines.
- The coverage test reports every violation in one run.

## Out-Of-Scope Artifact

A checked-in non-source file outside the source-code line limit because its
size comes from documentation, spec history, data provenance, generated
dependency state, or snapshots.

Fields:

- `path`: repository-relative path
- `reason`: spec, doc, JSON data, fixture, reference, golden, or generated
  lockfile

Rules:

- The mechanical test should not need exemptions for non-source extensions.
- Explicit source-code exemptions are limited to vendor-style third-party assets
  and reviewed allowlisted bundled third-party minified assets; maintained
  first-party minified source remains governed.

## Embedded Asset Part

A JS or CSS source fragment embedded into the binary and concatenated into the
single inline app asset at render time.

Fields:

- `path`: repository-relative path
- `kind`: JavaScript or CSS
- `order_key`: lexical filename order

Rules:

- Each part must be at or below 1000 lines.
- Parts concatenate in deterministic lexical order.
- The rendered report still receives one app JS block and one app CSS block.
