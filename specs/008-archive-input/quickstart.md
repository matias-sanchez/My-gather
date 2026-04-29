# Quickstart: Archive Input

## Validate Focused Tests

```bash
go test ./cmd/my-gather ./parse
```

## Validate Full Repository

```bash
go test ./...
```

## Generate From Directory

```bash
my-gather --overwrite \
  -o /tmp/report-CS0060148.html \
  /Users/matias/Documents/Incidents/CS0060148/eu-hrznp-d003/pt-stalk
```

## Generate From Archive

```bash
my-gather --overwrite \
  -o /tmp/report-CS0061180.html \
  /Users/matias/Documents/Incidents/CS0061180/evidence/raw/CS0061180_pt-stalk.zip
```

## Local Inventory Commands

```bash
find /Users/matias/Documents/Incidents \
  -path '*/code/*' -prune -o \
  -path '*/mysql-server/*' -prune -o \
  -type f -print |
awk '
function dirname(p){ sub("/[^/]+$", "", p); return p }
{
  n=$0; sub(".*/", "", n)
  if (n == "pt-summary.out" || n == "pt-mysql-summary.out" ||
      n ~ /^[0-9]{4}_[0-9]{2}_[0-9]{2}_[0-9]{2}_[0-9]{2}_[0-9]{2}-(iostat|top|variables|vmstat|meminfo|innodbstatus1|mysqladmin|processlist|netstat|netstat_s)$/)
    print dirname($0)
}' | sort -u > /tmp/my-gather-pt-stalk-candidate-dirs.txt

find /Users/matias/Documents/Incidents \
  -path '*/code/*' -prune -o \
  -path '*/mysql-server/*' -prune -o \
  -type f \( -iname '*.zip' -o -iname '*.tar' -o -iname '*.tar.gz' \
  -o -iname '*.tgz' -o -iname '*.gz' \) -print |
sort > /tmp/my-gather-archives.txt
```

Expected local inventory at feature creation:

- 171 pt-stalk candidate directories
- 560 archives

## Manual Validation

- Directory input still renders the known CS0060148 report.
- Archive input renders without manual extraction.
- `--verbose` shows extraction before parsing for archive inputs.
- Temporary extraction directories named `my-gather-input-*` are removed after
  the command exits.
- Unsupported archive extensions fail before parsing.
