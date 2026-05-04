# Research: File Size Constitution Guard

## Finding: Current Constitution Has No File-Size Rule

The constitution contains architectural and quality rules, including
library-first design and canonical code paths, but it does not currently set a
maximum source file length or prohibit source-code god files.

Decision: add a new core principle with a mechanical quality gate.

## Finding: Large Source Files

The governed first-party source code files over 1000 lines before this feature
are:

- `render/assets/app.js`
- `render/assets/app.css`

Decision: split JS/CSS into ordered embedded parts.

## Finding: Legitimate Large Data Artifacts

The repository contains specs, docs, JSON data, raw pt-stalk captures under
`_references/` and `testdata/`, plus golden snapshots and generated lockfiles.
These are not source-code god files and should not be rewritten for this source
line-count rule.

Decision: the mechanical test only considers source-code extensions, exempts
third-party minified assets under vendor-style directories, and requires an
explicit reviewed allowlist for bundled third-party minified assets outside
those directories. Maintained first-party minified source remains governed.

## Finding: Embedded Asset Ordering

The report currently embeds one app JS string and one app CSS string. Splitting
source files must not change the resulting inline report.

Decision: use a single Go helper that reads embedded asset part files in sorted
order and concatenates their bytes exactly.
