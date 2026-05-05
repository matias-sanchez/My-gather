# Changelog

All notable changes to my-gather are tracked here. Dates are UTC.

## v0.3.1 (2026-04-23)

Patch release for the feedback Worker path and report integration.

## v0.3.0 (2026-04-23)

Feedback Worker release: report submissions can create GitHub issues through
the project-controlled Cloudflare Worker, with the existing manual GitHub flow
preserved as the visible fallback path.

## v0.2.0 (2026-04-22)

Advisor engine + OS/DB enhancements pass on top of the v0.1.0 MVP.

### Added

- Advisor section: rule-based findings engine under `findings/`, with
  rules for buffer pool, binary logs, connections, query-shape
  heuristics, redo-log sizing, table cache, thread cache, and
  tmp-table activity; rendered as a fourth top-level section with a
  client-side severity filter.
- `-meminfo` collector and a dedicated Memory usage chart in the OS
  Usage section.
- Semaphore contention breakdown panel inside the InnoDB Semaphores
  card.
- Adaptive hash index (AHI) card now reports hit-ratio effectiveness
  instead of raw searches/sec.
- mysqladmin: floating counter-selection panel with a sticky strip +
  keyboard hotkey, category chips, custom dropdown replacing the
  native multi-select, one-click category loading.
- Variables: default filter is "Modified", identical snapshots
  collapse, rows that differ from the previous kept snapshot are
  highlighted, and MySQL 8.0 defaults are curated server-side so
  the "Changed" badge is deterministic.
- Charts: Y-axis unit badge on every chart; the series the cursor is
  pointing at is highlighted; fixed stacked-bar focus iteration
  direction.
- Report: iris-reveal nav drawer, favicon now uses the my-gather
  logo, noisy OS/DB "subviews" badges removed.
- Top CPU chart always includes mysqld.
- InnoDB: per-snapshot metrics aggregate into worst-value callouts.
- Multi-snapshot boundary markers on all time-series charts.

### Changed

- Top sections collapse by default on first load (prioritise signal
  over clutter); previous "expanded by default" state was cleared by
  bumping the `localStorage` namespace to v2.
- `Cmd/Ctrl + .` is now the nav-drawer keyboard accelerator (previously
  `Cmd/Ctrl + \`).
- `render/` split: single `render/render.go` fanned out into focused
  files (`assets.go`, `innodb.go`, `build_view.go`, `collection.go`,
  `concat.go`, etc.). Public surface narrowed to `Render`,
  `RenderOptions`, and `ErrNilCollection`; determinism helpers
  unexported.
- `findings`: `FormatNum` / `HumanInt` unexported; formatting is an
  internal concern.
- Report: Parser Diagnostics panel removed from the HTML; warnings
  and errors continue to mirror to stderr per FR-027.

### Fixed

- Variables: row weight in chips, Esc-key chaining, `inert` on the
  pinned mysqladmin panel, resize behaviour in the floating RM panel,
  `gofmt` cleanup across split `render/` files.
- A11y: focus rings on buttons and nav entries; keyboard reachability
  of the mysqladmin category chooser.

### Build / tooling

- `make lint` runs `gofmt -l` + `go vet ./...`.
- CHANGELOG.md added.

## v0.1.0 (2026-04-22)

Initial pt-stalk report MVP (feature 001). Supports seven collectors
(`-iostat`, `-top`, `-variables`, `-vmstat`, `-innodbstatus1`,
`-mysqladmin`, `-processlist`), produces one self-contained HTML file
with OS Usage, Variables, and Database Usage sections, byte-
deterministic output, zero network at runtime, and a committed
fixture + golden round-trip test per collector. CLI:
`my-gather <dir> -o <report.html>`.
