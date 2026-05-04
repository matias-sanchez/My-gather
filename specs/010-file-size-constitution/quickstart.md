# Quickstart: File Size Constitution Guard

## Validate File-Size Gate

```bash
go test -count=1 ./tests/coverage -run TestGovernedSourceFileLineLimit
```

## Validate Full Repository

```bash
go test -count=1 ./...
```

The Go test is the canonical governed-file scan. It owns the source extension
set, exact line-count semantics, vendor-style directory exemptions, and explicit
third-party bundled asset allowlist.
