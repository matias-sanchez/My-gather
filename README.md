# my-gather

A Go CLI that reads a [pt-stalk](https://docs.percona.com/percona-toolkit/pt-stalk.html) output directory and emits one self-contained HTML diagnostic report.

Built for MySQL / Percona support engineers and DBAs triaging incidents under time pressure.

## Status

Active development. See `specs/001-ptstalk-report-mvp/` for the current feature spec, implementation plan, and task list.

## Install

One-liner (macOS + Linux, amd64 + arm64) — verifies the SHA-256 against the published `SHA256SUMS` and installs into `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | sh
```

Pin to a specific tag or install somewhere else:

```bash
curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | VERSION=v0.3.0 sh
curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | PREFIX=/usr/local/bin sudo sh
```

Or grab a raw binary from the [latest release](https://github.com/matias-sanchez/My-gather/releases/latest) directly — `my-gather-darwin-arm64`, `my-gather-darwin-amd64`, `my-gather-linux-arm64`, `my-gather-linux-amd64`. Each tag also ships a `.tar.gz` that bundles the binary + README + CHANGELOG, plus a single `SHA256SUMS` covering every artifact.

## Quickstart

```bash
my-gather testdata/example2 -o /tmp/report.html
open /tmp/report.html   # macOS; xdg-open on Linux
```

Or build from source:

```bash
git clone git@github.com:matias-sanchez/My-gather.git
cd My-gather
make build
./bin/my-gather testdata/example2 -o /tmp/report.html
```

The report is one self-contained HTML file — CSS, JS, fonts, and data are inlined. Open it on an air-gapped laptop and everything renders.

## Design principles

The project's [constitution](.specify/memory/constitution.md) locks in twelve non-negotiables:

1. Ships as one statically-linked binary (no libc, no runtime install).
2. Read-only on pt-stalk inputs.
3. Graceful degradation — partial collections still produce useful reports.
4. Byte-deterministic HTML output.
5. Self-contained HTML (no CDN, no remote fetches at view time).
6. Library-first (`parse/`, `model/`, `render/` are independently importable).
7. Typed errors (sentinel + typed struct, not stringly-typed).
8. Every parser has a reference fixture + golden test.
9. Zero network at runtime.
10. Minimal dependencies (stdlib-first).
11. Reports optimized for humans under pressure (80% signal first).
12. Pinned Go version.

## Repo layout

```text
cmd/my-gather/   CLI entry point
parse/           Per-collector parsers + directory discovery
model/           Typed in-memory representation
render/          HTML templates + embedded assets + renderer
testdata/        Anonymised fixtures + golden outputs
tests/           Integration, coverage, performance tests
scripts/         Dev helpers (fixture anonymisation)
specs/           Feature specs, plans, tasks
_references/     Source material (real pt-stalk dumps, pt-mext C++ reference)
```

## Build & test

```bash
make test        # go test ./...
make vet         # go vet ./...
make build       # local binary -> ./bin/my-gather
make release     # cross-compile linux/{amd64,arm64} + darwin/{amd64,arm64} -> dist/
```

Goldens are regenerated with:

```bash
go test ./parse/... -update
```

The `-update` flag is registered only in packages that import `tests/goldens`; scope the invocation to goldens-using packages (currently `./parse/...`) and widen it as more packages adopt goldens. Review the resulting diff before committing.

## Scope (v1)

Supports eight pt-stalk collectors: `-iostat`, `-top`, `-variables`, `-vmstat`, `-innodbstatus1`, `-mysqladmin`, `-processlist`, `-meminfo`. Input collections up to 1 GB total / 200 MB per file. Output is HTML only (no JSON side-car in v1).

The rendered report has four top-level sections — OS Usage, Variables, Database Usage, and Advisor (rule-based findings derived from the parsed Collection). All four live in a single self-contained HTML file.

Unsupported input (older/forked pt-stalk formats, oversize collections, or output paths inside the input tree) is refused cleanly with a distinct non-zero exit code rather than silently partially written.

## Contributing

Read [`specs/001-ptstalk-report-mvp/quickstart.md`](specs/001-ptstalk-report-mvp/quickstart.md) for developer setup and the test / build / golden workflow.
