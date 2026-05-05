# Quickstart: Repository Alignment Audit

## Validate Go

```bash
go test -count=1 ./...
```

## Validate Worker

```bash
cd feedback-worker && npm run typecheck && npm test
```

## Validate Constitution Guard

```bash
scripts/hooks/pre-push-constitution-guard.sh
```

## Analyze Spec Artifacts

Run the Spec Kit analysis workflow for `specs/012-repo-alignment-audit/` and
confirm no high-severity spec/plan/tasks drift remains.
