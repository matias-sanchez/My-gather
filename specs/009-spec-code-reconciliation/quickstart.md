# Quickstart: Spec And Code Reconciliation

## Validate Focused Archive Tests

```bash
go test ./cmd/my-gather
```

## Validate Full Repository

```bash
go test ./...
```

## Check Active Feature Pointers

```bash
cat .specify/feature.json
sed -n '1,24p' AGENTS.md
sed -n '1,24p' CLAUDE.md
```

Expected active feature:

```text
009-spec-code-reconciliation
```

## Check Feature Status Lines

```bash
rg -n '^\*\*Status\*\*:' specs/*/spec.md
```

Expected highlights:

- `005-richer-processlist-metrics`: Complete
- `006-observed-slowest-queries`: Complete
- `007-advisor-intelligence`: Complete
- `008-archive-input`: Complete
- `009-spec-code-reconciliation`: Complete after validation passes
