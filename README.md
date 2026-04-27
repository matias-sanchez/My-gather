<div align="center">

<img src="docs/logo.png" alt="my-gather logo" width="260">

# my-gather

**Turn a [pt-stalk](https://docs.percona.com/percona-toolkit/pt-stalk.html) capture into a single, self-contained HTML diagnostic report.**

Built for MySQL / Percona support engineers and DBAs triaging incidents under time pressure.

[Install](#install) · [Quickstart](#quickstart) · [What you get](#what-you-get) · [Report feedback](#report-feedback) · [Design principles](#design-principles)

</div>

---

## Why

pt-stalk dumps drop a dozen files per snapshot — `iostat`, `top`, `vmstat`, `meminfo`, `variables`, `mysqladmin`, `innodbstatus1`, `processlist`, and more. At 3 AM on a production incident, opening them in a text editor is the wrong tool.

`my-gather` reads the capture once and emits **one HTML file** with:

- time-aligned charts across every collector,
- an **Advisor** section that runs rule-based checks (buffer pool vs workload, semaphore contention, pending flushes, etc.) and flags CRITICAL / WARNING / INFO / OK,
- a structured Environment panel (host + MySQL facts),
- collapsible variable snapshots with run-to-run deduplication,
- a **"Report feedback" button** wired to the project's GitHub Issues, so readers can file ideas without leaving the report.

The report is **fully self-contained**: CSS, JS, fonts, logo, and all the captured data are inlined. Open it on an air-gapped laptop, email it to a colleague, paste it into a ticket — it renders the same everywhere, with zero network calls.

## Install

**One-liner (macOS + Linux, amd64 + arm64)** — verifies SHA-256 against the published `SHA256SUMS` and installs into `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | sh
```

Pin a specific tag or change the install location:

```bash
curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | VERSION=v0.3.1 sh
curl -fsSL https://raw.githubusercontent.com/matias-sanchez/My-gather/main/scripts/install.sh | PREFIX=/usr/local/bin sudo sh
```

**Direct download**: grab a raw binary from the [latest release](https://github.com/matias-sanchez/My-gather/releases/latest) — `my-gather-darwin-arm64`, `my-gather-darwin-amd64`, `my-gather-linux-arm64`, `my-gather-linux-amd64`. Each tag also ships a `.tar.gz` that bundles the binary + README + CHANGELOG, plus a single `SHA256SUMS` covering every artifact.

**Build from source**:

```bash
git clone git@github.com:matias-sanchez/My-gather.git
cd My-gather
make build
./bin/my-gather --version
```

## Quickstart

Point at any pt-stalk output directory:

```bash
my-gather /path/to/pt-stalk-output -o /tmp/report.html
open /tmp/report.html           # macOS
xdg-open /tmp/report.html       # Linux
```

Or try it against the bundled anonymised fixture:

```bash
my-gather testdata/example2 -o /tmp/report.html --overwrite
open /tmp/report.html
```

Flags:

| Flag | Purpose |
|---|---|
| `-o, --out <path>` | Output HTML path (default `./report.html`). |
| `--overwrite` | Replace an existing output file. |
| `-v, --verbose` | Print per-file progress to stderr. |
| `--version` | Print version, commit, build date, platform. |
| `-h, --help` | Print usage and exit. |

Exit codes:

| Code | Meaning |
|---|---|
| `0` | Success. |
| `2` | Usage error (bad flag / missing arg). |
| `3` | Input path missing or unreadable. |
| `4` | Input is not a pt-stalk directory. |
| `5` | Input exceeds size bounds (1 GB total / 200 MB per file). |
| `6` | Output path exists (use `--overwrite`). |
| `7` | Output path resolves inside input directory (refusing to write). |
| `70` | Internal error — please open an issue. |

## What you get

The rendered report has these top-level sections, in this order:

1. **Environment** — host, kernel, CPU, memory, MySQL version, data-dir, key runtime variables. Time-stamped so you can see what drifted across snapshots.
2. **OS Usage** — CPU / memory / I/O / network charts, stacked by snapshot. Includes iostat, top, vmstat, meminfo, netstat + netstat-s.
3. **Variables** — dedup'd `SHOW GLOBAL VARIABLES` snapshots so you see *changes* rather than 400 rows of the same config repeated.
4. **Database Usage** — InnoDB status (semaphores, pending I/O, buffer pool, transactions), SHOW GLOBAL STATUS deltas, processlist summaries.
5. **Advisor** — the 80% signal: rule-based findings scored CRITICAL / WARNING / INFO / OK, grouped by subsystem (Buffer Pool, InnoDB Semaphores, Redo Log, Flushing, Binlog Cache, etc.) so you know where to look first.

Every chart is cross-hairable and every section is collapsible. The report stores your open/closed preferences in localStorage so the next time you open the same capture, it restores where you were.

### Report feedback

The header carries a **"Report feedback"** control. Clicking it opens a small dialog where you can type a title + body, pick a category (UI / Parser / Advisor / Other), paste a screenshot, or record a voice note (up to 10 min).

Submit posts the feedback directly to a [GitHub Issue](https://github.com/matias-sanchez/My-gather/issues) in the project repo, with the chosen category mapped to an `area/*` label. Attachments are uploaded to a Cloudflare R2 bucket and linked inline in the issue body — nothing leaves your browser before you click Submit.

If the backend is unreachable the dialog degrades gracefully to GitHub's new-issue URL with your title + body pre-filled.

## Design principles

The project's [constitution](.specify/memory/constitution.md) locks in fourteen non-negotiables:

1. **Single static binary.** No CGO, no libc version negotiation, no runtime install step.
2. **Read-only inputs.** Never modify the pt-stalk tree.
3. **Graceful degradation.** Partial captures still produce useful reports; parse failures become structured diagnostics, not panics.
4. **Deterministic output.** Same input + same binary → byte-identical HTML.
5. **Self-contained HTML.** All CSS, JS, fonts, and data embedded via `//go:embed`. No CDN, no external fonts, no runtime fetches for rendering.
6. **Library-first architecture.** `parse/`, `model/`, `render/` are independently importable without `package main`.
7. **Typed errors.** Sentinel + typed struct; `fmt.Errorf` without `%w` is banned for branchable conditions.
8. **Reference fixtures + golden tests.** Every parser ships with a fixture under `testdata/` and a committed `.golden`.
9. **Zero network at runtime** *(with one named exception: the Report Feedback Submit action posts to a project-controlled endpoint when the user explicitly clicks it)*.
10. **Minimal dependencies.** Stdlib-first; every third-party module justifies itself in the active feature's plan.
11. **Reports optimised for humans under pressure.** Primary narrative (trigger + state at trigger + deltas) takes precedence over exhaustive metric dumps.
12. **Pinned Go version.** Upgrades are their own reviewed commit.
13. **Canonical code path** *(NON-NEGOTIABLE)*. One implementation per behaviour — no silent fallbacks, no post-rename shims.
14. **English-only durable artifacts.** Code, comments, commit messages, specs, docs — all English. Exempt: `testdata/` and `_references/` (raw pt-stalk input).

## Repository layout

```text
cmd/my-gather/            CLI entry point
parse/                    Per-collector parsers + directory discovery
model/                    Typed in-memory representation
render/                   HTML templates + embedded assets + renderer
  ├── templates/          html/template sources
  └── assets/             app.css, app.js, chart.min.*, logo.png
findings/                 Advisor rules (one file per rule family)
feedback-worker/          Cloudflare Worker backend for the feedback button
testdata/                 Anonymised fixtures + golden outputs
tests/                    Integration, coverage, performance tests
scripts/                  Install script + dev helpers (fixture anonymisation, hooks)
specs/                    Spec-driven feature packages (spec + plan + tasks)
.specify/                 Constitution + speckit templates
.agents/                  Codex-facing local skills and agent context
.claude/                  Local Claude Code agents, skills, and hooks
docs/                     Project documentation assets (logo, etc.)
_references/              Source material (real pt-stalk dumps, pt-mext reference)
```

## Build & test

```bash
make test         # go test ./... -count=1
make vet          # go vet ./...
make build        # build ./bin/my-gather with version / commit / build-time ldflags
make release      # cross-compile linux/{amd64,arm64} + darwin/{amd64,arm64} -> dist/
```

### Golden regeneration

Parser goldens are committed alongside fixtures. Regenerate with:

```bash
go test ./parse/... -update
```

The `-update` flag is registered only in packages that import `tests/goldens`; widen the invocation carefully and always review the diff before committing. Golden changes should land as their own reviewed commit.

### Determinism check

A dedicated test renders the same fixture twice and asserts byte-identical output. If Principle IV is ever at risk, this test fails first.

## Scope (current)

**Collectors supported**: `-iostat`, `-top`, `-variables`, `-vmstat`, `-innodbstatus1`, `-mysqladmin`, `-processlist`, `-meminfo`, `-netstat`, `-netstat_s`, plus environment sidecars (`-hostname`, `-sysctl`, `-procstat`, `-meminfo-env`).

**Input size ceiling**: 1 GB total / 200 MB per file. Bigger captures are rejected with exit code 5.

**Output**: HTML only. No JSON side-car yet. The HTML bundle embeds every chart, icon, font, and data point.

**Format detection**: unsupported or forked pt-stalk variants are refused cleanly with exit code 4, rather than silently partially parsed.

## Contributing

1. Read the [constitution](.specify/memory/constitution.md). Every contribution gates on those fourteen principles.
2. New features go through the spec-driven pipeline — see `specs/` for prior examples (spec → plan → research → data-model → contracts → tasks → implement).
3. Local pre-push enforces the constitution via `scripts/hooks/pre-push-constitution-guard.sh` (wired through `.claude/settings.json`); CI runs the same checks on every PR.
4. Issues + discussions: the in-report "Report feedback" button is the preferred path; manual filing is also welcome at [github.com/matias-sanchez/My-gather/issues](https://github.com/matias-sanchez/My-gather/issues).

## License

See the project's LICENSE file for the full terms.
