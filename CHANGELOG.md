# Changelog

All notable changes to my-gather are tracked here. Dates are UTC.

## v0.4.0 (2026-05-07)

Six-feature roll-up since v0.3.1: streaming over the 1 GiB cap,
synchronized chart zoom + windowed legend stats, top-CPU caption +
mysqld pin, the InnoDB redo-log sizing panel, runtime theming, and a
required Author field on the feedback dialog. Plus the release
pipeline itself now sources GitHub Release bodies from this file.

### Added

- Synchronized chart zoom across the report: a single
  `__chartSyncStore` singleton drives shared zoom state, and
  per-series Min/Avg/Max in the legend recompute over the visible
  window. Stacked processlist dimensions get windowed stats from raw
  values; the HLL sparkline is canonically exempt (#51, #52, feature
  017).
- "Top CPU processes" caption ("Showing the top 3 processes by
  average CPU. When mysqld is running, it is always included, even
  when it is not in the top 3.") with an accurate aria-label and a
  regression test on the mysqld-pin invariant (#54, feature 018).
- InnoDB redo-log sizing panel with full MySQL version coverage:
  5.6 / 5.7 use `innodb_log_file_size` x `innodb_log_files_in_group`,
  8.0.30+ uses `innodb_redo_log_capacity`. Reports observed write
  rate (negative-delta clamp), 15-minute rolling peak with full-window
  guard, coverage minutes, 15-minute and 1-hour recommendations, and
  a warning when coverage is under 15 minutes. Cites Percona
  KB0010732 (#55, feature 019).
- Three themes selectable at runtime: light, dark (default), and
  Okabe-Ito color-blind-safe. Selection is persisted in
  `localStorage["my-gather:theme"]`. A single CSS-variable token
  block in `04.css` carries 16 distinct `--series-N` values per theme;
  `color-scheme` is set per theme. A pre-paint script in `<head>`
  applies the persisted theme before first paint to avoid FOUC. Live
  theme switches re-paint open charts via a `mygather:theme` event;
  the HLL sparkline has its own local listener; legend swatches use
  `var(--series-N)` so they re-resolve automatically (#53, feature
  020).
- Required Author field on the Report Feedback dialog. Submit is
  blocked while the trimmed value is empty. The last-used Author is
  persisted in localStorage on definitive worker success only
  (snapshot at submit time). The Worker rejects empty,
  whitespace-only, and over-cap payloads with typed errors. Issue
  bodies on both submission paths (worker and legacy `window.open`
  fallback) are prepended with "Submitted by: <name>" so triagers can
  attribute reports without scrolling git blame. The 80-character
  Author cap is expressed once in `feedback-contract.json` and
  consumed by the Go view, the JS form gate, and the TypeScript
  worker (#56, feature 021).

### Changed

- Removed the hard 1 GiB collection-size cap. The streaming invariant
  is now locked by a peak-heap-sampler regression test that parses
  1.19 GB of input at roughly 3 MiB resident heap. The spec-required
  64 GiB archive-extraction ceiling is preserved as a zip-bomb defence
  (#50, feature 016).
- Theming pass deletes parallel paths superseded by the runtime theme
  selector: the `prefers-color-scheme` media-query block, the
  `SERIES_COLORS` JavaScript array, and the hardcoded `#6b8eff`
  swatch are gone. Single source of truth is the CSS variable block
  in `04.css` (#53, feature 020).

### Build / tooling

- GitHub Release bodies are now sourced from this `CHANGELOG.md` via
  `scripts/release/extract-changelog-section.sh`, which the
  `publish-release` job in `.github/workflows/ci.yml` invokes to
  produce `dist/RELEASE_NOTES.md` and pass it as `body_path` to
  `softprops/action-gh-release@v2.0.8`.
- New CI job `changelog-gate` runs on tag pushes and fails the
  workflow if `CHANGELOG.md` has no `## vX.Y.Z (YYYY-MM-DD)` heading
  matching the pushed tag. `publish-release` declares
  `needs: [release, changelog-gate]` so a tag without a curated
  section cannot ship binaries (feature 022).
- Regression test under `tests/release/` exec's the extraction
  script against the in-repo `CHANGELOG.md` for both `v0.3.1` and
  `v0.4.0`, asserting the output is bounded by the next `## v`
  heading and that every feature attribution (016 through 021) is
  present in the v0.4.0 body (feature 022).

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
