# Quickstart: Full Compliance Fixes

Run from repository root.

## Spec Kit

```bash
.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks
```

## Go

```bash
go test -count=1 ./...
make lint
```

## Worker

```bash
cd feedback-worker
npm run typecheck
npm test
```

## Constitution Guard

```bash
CONSTITUTION_GUARD_CI=1 \
CONSTITUTION_GUARD_RANGE=origin/main..HEAD \
scripts/hooks/pre-push-constitution-guard.sh
```

## Canonical Path Searches

```bash
test ! -e .specify/scripts/bash/create-new-feature.sh
rg -n "ParseEnvMeminfo\\(" parse render reportutil tests feedback-worker .specify \
  --glob '!_references/**' --glob '!testdata/**' --glob '!.claude/worktrees/**'
rg -n "formatThousands|find_feature_dir_by_prefix|legacy fallback" \
  parse render reportutil tests feedback-worker .specify \
  --glob '!_references/**' --glob '!testdata/**' --glob '!.claude/worktrees/**'
```
