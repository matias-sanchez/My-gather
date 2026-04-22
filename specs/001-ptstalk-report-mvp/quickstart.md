# Quickstart: My-gather

Developer path from a freshly-cloned repo to a rendered report.

## Prerequisites

- Go (current stable; `go 1.24` or newer).
- `git`.
- A recent browser (Chromium-, Firefox-, or Safari-class, released in
  the last three years).

No other runtime dependencies. No Docker. No package managers. No
interpreter.

## One-time setup

```bash
git clone git@github.com:matias-sanchez/My-gather.git
cd My-gather
go version   # verify Go >= 1.24
```

## Run the test suite

```bash
go test ./...
```

All tests must pass. This exercises:

- Per-collector parser unit tests using fixtures under
  `testdata/example2/` (v1 ships a single curated example; a second
  fixture set under `testdata/example1/` for the preceding pt-stalk
  version, per FR-024, is a reserved slot — see tasks.md T019).
- Golden-file round-trips (parse → model → render → compare to
  `testdata/golden/*.html` byte-for-byte).
- A determinism test that renders twice and asserts the outputs
  differ only on the `GeneratedAt` line.
- The forbidden-import linter (Principle IX) checks that no
  shipped package imports `net/http`, `net/rpc`, etc.

### Updating golden files

When a deliberate change to the parsed-model JSON or the rendered
HTML lands, regenerate the golden files with:

```bash
go test ./parse/... -update
```

The `-update` flag is honoured by every golden-file test, but Go's
flag registration is per-test-binary: the flag is only recognised by
packages that import `tests/goldens`. `go test ./... -update` fails
with `flag provided but not defined: -update` in packages that do
not use goldens (render, tests/lint, tests/coverage today). Scope
the invocation to goldens-using packages and widen the scope as
more adopt goldens. Review the resulting diff carefully; a
one-character rendering change can invalidate every golden file in
the tree.

## Build the binary

Local build:

```bash
go build -o ./bin/my-gather ./cmd/my-gather
```

Cross-compile release-style binaries for all four supported targets:

```bash
make release     # or: .github/scripts/build.sh
```

This sets `CGO_ENABLED=0` and cross-compiles for `linux/{amd64,arm64}`
and `darwin/{amd64,arm64}`, writing artifacts to `dist/`.

## Run against a reference collection

Always put flags before the positional `<input-dir>` — the Go stdlib
`flag` package stops at the first non-flag token:

```bash
./bin/my-gather --out /tmp/report.html testdata/example2
open /tmp/report.html      # macOS; on Linux use: xdg-open /tmp/report.html
```

The report should open with three sections (OS Usage, Variables,
Database Usage) and all graphs rendered. Disconnect your laptop from
the network and reload — the report must continue to render
identically.

With `-v` for progress:

```bash
./bin/my-gather -v --out /tmp/report.html testdata/example2
```

## Exit codes to expect

- `0` — success, `/tmp/report.html` written.
- `6` — `/tmp/report.html` already exists; re-run with `--overwrite`
  or point at a different path.
- `3`, `4`, `5` — input path / not-pt-stalk / size-bound errors (see
  `specs/001-ptstalk-report-mvp/contracts/cli.md`).

## Verifying determinism locally

```bash
./bin/my-gather --out /tmp/a.html --overwrite testdata/example2
./bin/my-gather --out /tmp/b.html --overwrite testdata/example2
diff /tmp/a.html /tmp/b.html
```

The diff should show exactly one line of difference — the
"Report generated at" timestamp. Any other diff is a determinism
regression (Principle IV) and must be fixed before merging.

## Where things live

- `cmd/my-gather/` — CLI entry point. Keep thin.
- `parse/` — per-collector parsers + discovery.
- `model/` — typed representation shared by parse and render.
- `render/` — HTML templates + embedded assets + rendering pipeline.
- `testdata/` — anonymised fixtures + golden outputs.
- `_references/examples/` — source material for `testdata/`;
  regenerate anonymised fixtures with
  `scripts/anonymise-fixtures.sh <example-dir>`.
- `specs/001-ptstalk-report-mvp/` — this feature's spec, plan,
  research, data model, contracts.
- `.specify/memory/constitution.md` — the 12 principles the
  implementation is measured against.

## Adding a new parser (for future features)

Per Constitution Principle VIII, new parsers MUST ship with a fixture
and a golden file. Checklist:

1. Add a `SuffixFoo` constant to `model.Suffix`.
2. Add `model.FooData` with a concrete payload shape (see
   `specs/001-ptstalk-report-mvp/data-model.md` for the style).
3. Add `parse/foo.go` with a `parseFoo(…)` routine that returns
   `(*model.FooData, []model.Diagnostic)`.
4. Add fixtures under `testdata/example*/…-foo` (at least one per
   supported pt-stalk version).
5. Add a golden file under `testdata/golden/` via `go test -update`.
6. Add a render section or extend an existing one in `render/`.
7. Run `go test ./...` and verify the determinism test passes.

The `testdata_coverage_test.go` guard fails the build if a `Suffix`
constant exists with no fixture, closing the door on forgotten
steps.

## The `-mysqladmin` delta algorithm (ported from pt-mext)

The `-mysqladmin` parser implements its delta computation natively in
Go (`parse/mysqladmin.go`), ported from the C++ reference at
`_references/pt-mext/pt-mext-improved.cpp`. The C++ file is retained
as historical reference and reading material; **it is never compiled
or invoked during build or test.**

Correctness is anchored by a committed golden fixture:

- `testdata/pt-mext/input.txt` — verbatim copy of the worked input
  table from the comment at lines 146–177 of the C++ file.
- `testdata/pt-mext/expected.txt` — verbatim copy of the expected
  output lines at 181–189 of the C++ file.

The test `parse/mysqladmin_test.go::TestPtMextFixture` runs the Go
parser on `input.txt` and asserts byte-for-byte equality with
`expected.txt` for counter-variable aggregates. Since the C++ author
authored the example and expected output together, this gives us
pt-mext's correctness guarantee without requiring a C++ toolchain.

**Deliberate deviations from the C++ reference** (see research R8):

- **Counter vs. gauge classification**: `Threads_running`, `Uptime`,
  `Open_tables`, etc. are gauges and stored as raw per-sample values,
  not deltas.
- **`avg` uses `float64`** rather than integer division, so small
  deltas don't truncate to zero.
- **Variables appearing/disappearing mid-collection** emit a
  warning diagnostic and store `NaN` at the missing slot.

Run `go test ./parse/...` whenever you modify `parse/mysqladmin.go`.
