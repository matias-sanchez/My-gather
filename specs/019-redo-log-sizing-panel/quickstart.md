# Quickstart: Redo log sizing panel

**Feature**: 019-redo-log-sizing-panel
**Audience**: My-gather contributors verifying the new panel locally.

## Build

```sh
go build ./...
```

## Test

Run the new panel's focused tests:

```sh
go test ./render/ -run TestComputeRedoSizing -v
```

Run the full suite to confirm no regression in surrounding render
or determinism tests:

```sh
go test ./...
```

Run `go vet`:

```sh
go vet ./...
```

## Manual verification

Generate a report from a real pt-stalk capture and open the resulting
HTML:

```sh
go run ./cmd/my-gather -in <pt-stalk-capture-dir> -out /tmp/report.html
open /tmp/report.html
```

Open the **Database Usage** section, then the **InnoDB status** subsection.
The new **Redo log sizing** panel renders immediately under the existing
InnoDB callouts, with:

- Configured redo space (e.g. `2.0 GiB`).
- Source label (`innodb_redo_log_capacity` or
  `innodb_log_file_size x innodb_log_files_in_group`).
- Observed write rate as `bytes/sec` and `bytes/min`.
- Peak window descriptor (e.g. `15-minute` or `available 28-second`).
- Coverage estimate in minutes.
- Recommended sizes for `15 minutes of peak` and `1 hour of peak`.
- A warning line when coverage is below 15 minutes.
- A citation of `Percona KB0010732` as the methodology source.

## Pre-push gate

The constitution pre-push hook runs from the repo root and is wired
via `.claude/settings.json`:

```sh
.git/hooks/pre-push
```

Or invoke the hook script directly:

```sh
scripts/hooks/pre-push-constitution-guard.sh
```
