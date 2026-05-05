# Quickstart: Reconciliation Cleanup

Run from repository root on branch `014-reconciliation-cleanup`.

## Spec Kit

```bash
.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks
```

Run the repo-local `/speckit-analyze` workflow against this active feature and
confirm it reports no blocking contradictions.

## Go

```bash
go test -count=1 ./...
make lint
```

## Worker Repo Health

```bash
cd feedback-worker
npm run typecheck
npm test
```

Run these as full-repository health checks even when this branch does not
change Worker files.

## Constitution Guard

```bash
CONSTITUTION_GUARD_CI=1 \
CONSTITUTION_GUARD_RANGE=origin/main..HEAD \
scripts/hooks/pre-push-constitution-guard.sh
```

## Canonical Path Searches

```bash
test ! -e .specify/extensions/git/scripts/bash/git-common.sh
rg -n "minimal fallback|source checkout fallback|_PROJECT_ROOT/scripts/bash/common.sh|SCRIPT_DIR/git-common.sh|scripts/powershell/common.ps1" \
  .specify/extensions/git/scripts
rg -n "ParseEnvMeminfo\\(|formatThousands|find_feature_dir_by_prefix" \
  parse render reportutil tests feedback-worker .specify \
  --glob '!_references/**' --glob '!testdata/**' --glob '!.claude/worktrees/**'
```
