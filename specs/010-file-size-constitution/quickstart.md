# Quickstart: File Size Constitution Guard

## Validate File-Size Gate

```bash
go test ./tests/coverage -run TestGovernedSourceFileLineLimit
```

## Validate Full Repository

```bash
go test -count=1 ./...
```

## Check Governed Files Manually

```bash
git ls-files | while IFS= read -r f; do
  case "$f" in
    _references/*|testdata/*|render/assets/chart.min.*) continue ;;
  esac
  case "$f" in
    *.go|*.js|*.css|*.ts|*.tsx|*.jsx|*.sh|*.tmpl) ;;
    *) continue ;;
  esac
  [ -f "$f" ] || continue
  lines=$(wc -l < "$f" | tr -d ' ')
  [ "$lines" -gt 1000 ] && printf '%6d %s\n' "$lines" "$f"
done
```

Expected output: none.
