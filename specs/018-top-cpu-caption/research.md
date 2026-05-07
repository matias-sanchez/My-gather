# Research: Top CPU Processes Caption

## Question 1 — Does the mysqld pin already exist?

**Decision**: Yes. `render/concat.go::concatTop` already implements
the pin. After ranking PIDs by average CPU and selecting the top 3,
the function checks whether any of those three matches
`isMysqldCommand` (case-insensitive `mysqld` or `mariadbd`, trimmed).
If none of the top-3 is mysqld, it walks the remaining ranked PIDs
and appends the first matching mysqld series to `Top3ByAverage`.

**Evidence** (lines 196-215 of `render/concat.go`):

```go
// Always surface mysqld, even when it isn't in the global top-3.
mysqldAlreadyIn := false
for i := 0; i < limit; i++ {
    if isMysqldCommand(seriesByPID[pidsRanked[i]].Command) {
        mysqldAlreadyIn = true
        break
    }
}
if !mysqldAlreadyIn {
    for _, pid := range pidsRanked[limit:] {
        if isMysqldCommand(seriesByPID[pid].Command) {
            out.Top3ByAverage = append(out.Top3ByAverage, *seriesByPID[pid])
            break
        }
    }
}
```

**Rationale**: The pin is the load-bearing reason the caption can
truthfully say "mysqld is always included". Because it is already
implemented, this feature does not modify chart-data preparation.

**Alternatives considered**: Reimplementing the pin in the renderer
or in a new helper. Rejected — the canonical-code-path principle
(XIII) forbids parallel implementations, and the existing one is
correct and tested.

## Question 2 — Does a regression test for the pin already exist?

**Decision**: Yes. `render/concat_test.go` contains
`TestConcatTopRecomputesTop3WithMysqldAlwaysIn` and
`TestConcatTopMysqldAlreadyInTopThree`. Both exercise exactly the
invariant the caption asserts.

**Implication for this feature**: A new regression test is still
added per FR-005, but it focuses specifically on the chart's
*series list as exposed to the chart payload* (i.e., that
`Top3ByAverage` — the field the renderer reads to build the chart —
contains mysqld even when mysqld is not in the top 3). This is
phrased to lock the caption's promise rather than the merge
algorithm's internal behaviour, even though both happen to live in
the same field today. If the renderer ever changes which field
feeds the chart, this test must move with it.

## Question 3 — Where does the caption belong?

**Decision**: Inline inside `render/templates/os.html.tmpl`, in the
`sub-os-top` `<details>` block, immediately above the chart `<div>`,
gated by the same `if .HasTop` condition.

**Rationale**: The OS section template already places inline `<p>`
banners and `<div class="chart-summary">` blocks directly in the
template. There is no existing partial for chart captions, and
introducing one for a single caption would create a new path with
nothing else flowing through it (smell: speculative abstraction
forbidden by Principle XIII).

**Alternatives considered**:
- New partial template `caption.html.tmpl`. Rejected — single
  caller, no reuse, adds a competing path.
- JavaScript-injected caption in `app-js`. Rejected — caption is
  static text; HTML template is the canonical path for static
  content; injecting via JS would also break `<noscript>` users.

## Question 4 — Caption wording

**Decision**: "Showing the top 3 processes by average CPU. When mysqld
is running, it is always included, even when it is not in the top 3."

**Rationale**: Two short sentences, no jargon. Says explicitly
"average CPU" because the chart-summary stats already use "avg" and
the merge code ranks by average. Says "even when it is not in the
top 3" rather than "regardless of rank" because the former tells the
reader exactly what they need to know without requiring them to
reason about ranking semantics. The "When mysqld is running" qualifier
keeps the caption unconditionally true on hosts where mysqld is not
running — the pin in `concatTop` only fires when a `mysqld`/`mariadbd`
process exists in the capture.

**Alternatives considered**:
- Issue-suggested wording "Showing top 3 processes by CPU. mysqld is
  always included." — accepted as semantically equivalent; the
  chosen wording is a slightly more precise variant. Either would
  satisfy the spec's "or equivalent wording" clause.

## Question 5 — Caption styling

**Decision**: Use a small `<p class="chart-caption">` element. Add
one CSS rule for `.chart-caption` to the appropriate ordered CSS
source part under `render/assets/app-css/` if no existing class
fits.

**Rationale**: The OS section already uses `class="banner"` for
warning banners and `class="chart-summary"` for stat strips. Neither
fits a neutral explanatory caption. A single new utility class keeps
the styling explicit and reusable should other charts get captions
later.
