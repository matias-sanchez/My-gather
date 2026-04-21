#!/usr/bin/env bash
#
# anonymise-fixtures.sh — deterministic fixture anonymisation.
#
# Reads a pt-stalk collection directory under _references/examples/ and
# writes an anonymised copy to testdata/<name>/ per research R6.
#
# Usage:
#   scripts/anonymise-fixtures.sh <source-dir> <dest-dir>
#
# Example:
#   scripts/anonymise-fixtures.sh _references/examples/example2 testdata/example2
#
# The script is deterministic: re-running with the same inputs produces
# byte-identical outputs. It uses only bash, awk, and sed — no python,
# no jq, no third-party tools.

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <source-dir> <dest-dir>" >&2
  exit 2
fi

src="$1"
dst="$2"

if [[ ! -d "$src" ]]; then
  echo "source directory not found: $src" >&2
  exit 3
fi

# Seven source-file suffixes this feature supports (per spec FR-001..FR-023
# and the Suffix enum in model/model.go). Anonymise only these plus the
# summary files used for directory-recognition fallback.
SUFFIXES=(
  iostat top variables vmstat innodbstatus1 mysqladmin processlist
  hostname
)

mkdir -p "$dst"

# Build the deterministic substitution stream. Each rule is applied in
# order. Rules are intentionally coarse — structural fields (timestamps,
# numeric counts, variable names, process IDs, state labels) are left
# intact because the parsers depend on them.
#
# The script itself is the source of truth for these rules; update the
# table in research.md R6 when you change them.
anonymise() {
  sed -E \
    -e 's/\b([a-zA-Z0-9_-]+)\.internal\.example\b/example-db-01.internal.example/g' \
    -e 's/\b([a-zA-Z0-9_-]+)\.ec2\.internal\b/example-db-01.ec2.internal/g' \
    -e 's/\b([a-zA-Z0-9_-]+)\.compute\.amazonaws\.com\b/example-db-01.compute.amazonaws.com/g' \
    -e 's/\bip-[0-9]+-[0-9]+-[0-9]+-[0-9]+\b/ip-10-0-0-1/g' \
    -e 's/\b10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b/192.0.2.10/g' \
    -e 's/\b172\.(1[6-9]|2[0-9]|3[0-1])\.[0-9]{1,3}\.[0-9]{1,3}\b/192.0.2.11/g' \
    -e 's/\b192\.168\.[0-9]{1,3}\.[0-9]{1,3}\b/192.0.2.12/g' \
    -e 's/\/home\/[a-zA-Z0-9_-]+\//\/home\/redacted\//g'
}

count=0
shopt -s nullglob

for suffix in "${SUFFIXES[@]}"; do
  for f in "$src"/*-"$suffix"; do
    [[ -e "$f" ]] || continue
    base="$(basename "$f")"
    anonymise < "$f" > "$dst/$base"
    count=$((count + 1))
  done
done

# Copy the summary files if present — they're used for directory
# recognition (research R5).
for name in pt-summary.out pt-mysql-summary.out; do
  if [[ -f "$src/$name" ]]; then
    anonymise < "$src/$name" > "$dst/$name"
    count=$((count + 1))
  fi
done

echo "[anonymise] $count files written to $dst"
