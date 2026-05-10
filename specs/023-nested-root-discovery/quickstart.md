# Quickstart: Nested Root Discovery for Directory Inputs

**Feature**: 023-nested-root-discovery
**Date**: 2026-05-08

This document is the operator-facing walkthrough for the change.
After this feature ships, the following layouts work; before, they
did not.

## What changed in one sentence

`my-gather <directory>` now finds the pt-stalk capture even when
it lives in a subdirectory of `<directory>`, the same way archive
input has always worked.

## What now works

### Layout 1: Capture at the top level (unchanged)

```text
my-pt-collection/
├── 2026_04_03_14_21_14-mysqladmin
├── 2026_04_03_14_21_14-iostat
├── 2026_04_03_14_21_14-vmstat
└── ...
```

```bash
my-gather my-pt-collection -o report.html
```

Behaviour: bit-identical to today. No subdirectory walk runs.

### Layout 2: Capture nested in a single subdirectory

```text
case-folder/
└── host-name/
    ├── 2026_04_03_14_21_14-mysqladmin
    └── ...
```

```bash
my-gather case-folder -o report.html
```

Behaviour after this feature: the tool walks the input tree, finds
`case-folder/host-name/`, and renders it.

Today: `case-folder is not recognised as a pt-stalk output directory`.

### Layout 3: Capture deeply nested under `tmp/pt/collected/<host>/`

```text
case-folder/
└── host-logs/
    └── host/
        └── tmp/
            └── pt/
                └── collected/
                    └── host/
                        ├── 2026_04_03_14_21_14-mysqladmin
                        └── ...
```

```bash
my-gather case-folder -o report.html
```

Behaviour after this feature: the tool walks 6 levels in to find the
capture and renders it. This is the dominant real-world layout for
attached pt-stalk collections in the operator's local corpus.

## What still fails (with a clearer message)

### Multi-host case folder

```text
multi-host-case/
├── host-A/
│   └── tmp/pt/collected/host-A/
│       └── 2026_04_03_14_21_14-mysqladmin
└── host-B/
    └── tmp/pt/collected/host-B/
        └── 2026_04_03_14_21_14-mysqladmin
```

```bash
my-gather multi-host-case -o report.html
```

Stderr (deterministic, lexically ordered; root paths are absolute):

```text
my-gather: /abs/path/to/multi-host-case contains multiple pt-stalk collections:
  /abs/path/to/multi-host-case/host-A/tmp/pt/collected/host-A
  /abs/path/to/multi-host-case/host-B/tmp/pt/collected/host-B
re-run pointing at one of these paths
```

Exit code: 4 (`exitNotAPtStalkDir`). No output file is written.

### Folder with no pt-stalk capture anywhere

```bash
my-gather random-folder -o report.html
```

Stderr:

```text
my-gather: random-folder is not a pt-stalk output directory and no pt-stalk collection was found in its subdirectories (searched up to depth 8)
```

Exit code: non-zero. No output file is written.

## How to verify locally

The repository ships a synthetic fixture used by the integration
tests. Operators do not normally invoke it directly, but it is the
fastest path to a hands-on demo:

```bash
go test ./cmd/my-gather/ -run TestCLIDirInputNestedSingle -v
```

For a real-world spot check against the operator's local capture
corpus, the fastest example is:

```bash
my-gather "/Users/matias/Documents/Incidents/CS0061420" -o /tmp/c61420.html
```

(`CS0061420` has the deepest layout in the corpus: 7 directories
between the case folder and the actual pt-stalk root. Note: this
case has multiple host folders, so the expected outcome is the
"multiple roots" stderr block listing three roots, not a successful
report. To get a successful report, point at one host:)

```bash
my-gather "/Users/matias/Documents/Incidents/CS0061420/DBCLTIFT01PDC_logs" -o /tmp/c61420-host1.html
```

## Operator-facing release-note line

> `my-gather <dir>` now auto-discovers the pt-stalk capture inside
> common nested layouts (e.g. `case/host/tmp/pt/collected/host/`).
> If the directory contains more than one pt-stalk root, the tool
> exits with a list of all candidate paths and asks the operator to
> pick one.

## Caveats

- The walk is bounded at 8 directory levels below the input. If a
  layout puts the capture deeper than that, point the tool at any
  intermediate directory closer to the capture.
- Hidden directories (any name starting with `.`) are skipped.
  Operators do not store pt-stalk captures in hidden directories;
  this filter exists to keep the walk fast on case folders that
  contain `.git`, `.cache`, etc.
- Symbolic links to other directories are not followed. A
  symlinked pt-stalk root works only if it is the input directory
  itself; the walker does not chase symlinks inside the tree.
