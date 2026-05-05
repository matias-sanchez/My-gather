# Quickstart: Compliance Closure

## Validation

```bash
go test -count=1 ./...
make lint
scripts/hooks/pre-push-constitution-guard.sh
cd feedback-worker && npm run typecheck && npm test
```

## Coverage And Audit Checks

```bash
go test -coverprofile=/tmp/my-gather-cover.out ./...
go tool cover -func=/tmp/my-gather-cover.out | tail -1

find . -path './.git' -prune -o \
  -path './feedback-worker/node_modules' -prune -o \
  -path './.claude/worktrees' -prune -o \
  -path './testdata' -prune -o \
  -path './_references' -prune -o \
  -type f \( -name '*.go' -o -name '*.sh' -o -name '*.ps1' -o \
  -name '*.js' -o -name '*.css' -o -name '*.html' -o \
  -name '*.tmpl' -o -name '*.ts' -o -name '*.tsx' -o \
  -name '*.jsx' \) -print0 |
  xargs -0 wc -l |
  awk '$2 != "total" && $1 > 1000 {print}'
```
