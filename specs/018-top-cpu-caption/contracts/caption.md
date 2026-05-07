# Contract: Top CPU Processes Caption

## Template Contract

**Location**: `render/templates/os.html.tmpl`, inside the
`sub-os-top` `<details>` element, inside its `body` `<div>`,
inside the `if .HasTop` branch, immediately above the
`<div class="chart" id="chart-top" ...>` element.

**Markup** (exact text shipped):

```html
<p class="chart-caption">Showing the top 3 processes by average CPU. When mysqld is running, it is always included, even when it is not in the top 3.</p>
```

**Gating**: The caption MUST share the same `{{- if .HasTop }}`
branch as the chart container. When `.HasTop` is false, neither
the chart nor the caption renders.

**Uniqueness**: The caption text MUST appear exactly once in the
generated report. A renderer-level test asserts this.

## Chart-Series Pin Contract (existing, locked)

**Location**: `render/concat.go::concatTop`.

**Invariant**: For every input where `concatTop` returns a non-nil
`*model.TopData`, if any merged process sample's `Command` matches
`isMysqldCommand` (i.e., trimmed and case-insensitive `mysqld` or
`mariadbd`), then the returned `Top3ByAverage` slice MUST contain a
`ProcessSeries` whose `Command` matches `isMysqldCommand`.

**Tests**:
- `render.TestConcatTopRecomputesTop3WithMysqldAlwaysIn` (existing).
- `render.TestConcatTopMysqldAlreadyInTopThree` (existing).
- New: `render.TestConcatTopMysqldPinnedWhenLowest` — explicitly
  named after the caption's promise; constructs a snapshot stream
  where mysqld is the *lowest*-CPU process and asserts mysqld
  appears in `Top3ByAverage` after merge.

## Caption-Presence Test Contract

**Location**: `render/os_test.go` (new test).

**Behavior**: Render an OS section with a non-empty top capture;
assert the rendered HTML contains the exact caption string above.
Also render the same section with `HasTop` false (no top data) and
assert the caption string does NOT appear.
