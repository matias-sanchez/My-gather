# Feature 001 Retrospective: `ptstalk-report-mvp`

Record of the manual quickstart walkthrough (T085) plus any friction
surfaced along the way. Each entry cites the `quickstart.md` step that
produced it; entries are either `resolved` (a fix landed in the same
PR as this retrospective) or `follow-up` (a new task added to
`tasks.md` for a later cut).

Walkthrough performed 2026-04-22 against commit on branch
`claude/perf-quality-bundle-f`, built from a freshly-cloned checkout
of `matias-sanchez/My-gather`.

## One-time setup

**Step**: `go version` verifying Go ≥ 1.24.

**Observed**: `go version go1.24.7 linux/amd64` — clean.

**Friction**: none.

## Run the test suite

**Step**: `go test ./...`.

**Observed**: all packages green on the current feature branch. The
integration-test package builds the binary once per invocation
(`tests/integration/bin_test.go::TestMain`), so the first run adds
~1 s of compile time; subsequent runs within the same `go test`
invocation reuse the cached binary.

**Friction**: none blocking. The perf tests (`tests/perf/...`) run
their 5 MB sanity case by default and take ~0.1 s; the 50 MB and
100 MB SC-007 / SC-001 variants are gated behind `testing.Short()`
and add ~2 s total when the full non-short suite runs. This is
within the spec's 10 s + 60 s envelopes but worth documenting so CI
runners don't accidentally run the short suite.

## Updating golden files

**Step**: `go test ./parse/... -update` (and for render goldens,
`go test ./render/... -update`).

**Observed**: runs cleanly; `testdata/golden/` updates in place
with the expected diff.

**Friction**: `go test ./... -update` (no package scope) fails
with `flag provided but not defined: -update` in packages that
don't import `tests/goldens` (today: `render` from before bundle B,
`tests/lint`, `tests/coverage`). `quickstart.md` already documents
this — the scope-it-to-goldens-using-packages workaround is
correct. **Follow-up (minor)**: consider registering a no-op
`-update` flag at the root module level so the unscoped form
doesn't error, or update `quickstart.md`'s "regenerate the
golden files" snippet to explicitly scope by package. Not worth
blocking v1 on.

## Build the binary

**Step**: `go build -o ./bin/my-gather ./cmd/my-gather`.

**Observed**: builds in < 3 s on a cold cache.

**Friction**: the walkthrough used `go build -o /tmp/qs_retro/
my-gather ./cmd/my-gather` instead of the quickstart's
`./bin/my-gather` to avoid polluting the repo tree; both shapes
work.

## `--version` output

**Step**: `./bin/my-gather --version`.

**Observed**:
```
my-gather v0.0.0-dev
  commit:   unknown
  go:       go1.24.7
  built:    unknown
  platform: linux/amd64
```

**Friction**: `commit` and `built` fields read "unknown" when the
binary is built with a plain `go build` rather than via
`make release` (which injects `-ldflags` with the commit hash and
build date). This is by design — the placeholder value is a
signal that the binary was a developer build, not a release — but
it is worth calling out in `quickstart.md` so a first-time reader
doesn't think the version line is broken. **Resolved**: noted here;
no code change required.

## Run against a reference collection

**Step**: `./bin/my-gather --out /tmp/report.html testdata/example2`.

**Observed**: exits 0; `/tmp/report.html` written at 765 KB. Report
opens cleanly in a browser with three sections rendered.

**Friction**: none. The positional-arg rule (flags must precede the
input directory) is documented in `quickstart.md` and consistent
with `flag.Parse()` semantics.

## Verifying determinism locally

**Step**:
```
./bin/my-gather --out /tmp/a.html --overwrite testdata/example2
./bin/my-gather --out /tmp/b.html --overwrite testdata/example2
diff /tmp/a.html /tmp/b.html
```

**Observed**: `diff` produces no output — both renders are
byte-identical. This is STRONGER than the quickstart's "one line
of difference (the `Report generated at` timestamp) is expected"
wording, because the renderer accepts `--generated-at` via
`render.RenderOptions.GeneratedAt` and the two calls within one
second likely produced identical times on a fast machine.

**Friction**: `quickstart.md` says to expect "exactly one line of
difference". On machines fast enough to complete both runs within
a single second, the diff is actually EMPTY. Both shapes are
within spec (Principle IV allows the `GeneratedAt` line to vary),
but the doc wording could confuse a reviewer who sees the empty
diff and thinks something went wrong. **Follow-up (trivial)**:
amend `quickstart.md` to say "up to one line of difference". Fit
for a later doc-only PR.

## Exit codes

**Step**: re-run without `--overwrite` to exercise exit 6.

**Observed**: exits with code 6, stderr reads `my-gather: output
file /tmp/a.html already exists; re-run with --overwrite or
choose a different --out path`.

**Friction**: none. The exit code + stderr message match
`contracts/cli.md`.

## Summary

Zero blocking friction, three doc-level follow-ups (all trivial,
none impact the shipping v1 contract):

1. `quickstart.md` could mention that `commit` / `built` read
   "unknown" for non-release builds.
2. `quickstart.md`'s determinism-check wording assumes one-line
   diff; empty diff is valid too.
3. Consider a root-level `-update` flag registration so
   `go test ./... -update` (unscoped) does not error — or update
   the doc to always scope by package.

The v1 UX is cut-ready. No parser, render, or CLI bug surfaced
during the walk.

---
Generated 2026-04-22 as part of T085.
