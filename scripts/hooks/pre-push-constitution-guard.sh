#!/usr/bin/env bash
# Pre-push constitution guard for My-gather.
#
# Fast mechanical checks that run before `git push` via Claude Code's
# PreToolUse hook. This catches the highest-signal constitution violations
# locally so the codex-review loop converges in 1-2 rounds instead of 8.
#
# Scope is intentionally narrow: only rules that are cheap, deterministic,
# and low-false-positive at the diff level. Semantic review lives in the
# pre-review-constitution-guard subagent, not here.
#
# Contract:
#   stdin:   Claude Code hook JSON (ignored; we compute the diff ourselves)
#   stdout:  empty on pass, or a JSON PreToolUse decision blocking the push
#   exit:    always 0 (Claude Code reads the decision from stdout JSON)
#
# Requires: git, jq.

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
  exit 0
fi
cd "$REPO_ROOT"

LOG="$REPO_ROOT/.claude/pre-review.log"
mkdir -p "$(dirname "$LOG")"

BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
UPSTREAM="$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)"
if [ -n "$UPSTREAM" ]; then
  RANGE="$UPSTREAM..HEAD"
else
  RANGE="origin/main..HEAD"
fi

COMMIT_COUNT="$(git rev-list --count "$RANGE" 2>/dev/null || echo 0)"
if [ "$COMMIT_COUNT" -eq 0 ]; then
  exit 0
fi

CHANGED="$(git diff --name-only "$RANGE" 2>/dev/null || true)"

VIOLATIONS=()

# Rule 1 (Principle VIII): a new top-level collector parser under parse/
# MUST ship with a fixture and a golden in the same push. Scoped to
# the basenames that match model.KnownSuffixes — helper parsers
# (envhostname, sysctl, topheader, envmeminfo, procstat, dfsnapshot,
# version, parse, …) consume snippets embedded in other collectors'
# files and have no top-level fixture of their own.
COLLECTOR_PARSERS=(
  iostat
  top
  vmstat
  meminfo
  variables
  innodbstatus
  mysqladmin
  processlist
  netstat
  netstat_s
)
NEW_PARSERS="$(git diff --name-only --diff-filter=A "$RANGE" -- 'parse/*.go' ':!parse/*_test.go' 2>/dev/null || true)"
for f in $NEW_PARSERS; do
  base="$(basename "$f" .go)"
  is_collector=0
  for c in "${COLLECTOR_PARSERS[@]}"; do
    if [ "$base" = "$c" ]; then
      is_collector=1
      break
    fi
  done
  if [ "$is_collector" -eq 0 ]; then
    continue
  fi
  if ! printf '%s\n' "$CHANGED" | grep -qE "^testdata/.*${base}"; then
    VIOLATIONS+=("VIII: new parser $f added without a matching fixture under testdata/ (expected testdata/*${base}*)")
  fi
  if ! printf '%s\n' "$CHANGED" | grep -qE "^testdata/golden/.*${base}"; then
    VIOLATIONS+=("VIII: new parser $f added without a golden under testdata/golden/ (expected testdata/golden/*${base}*)")
  fi
done

# Rule 2 (Principle X): a new direct dependency in go.mod MUST be
# justified in the active feature's plan.md.
if printf '%s\n' "$CHANGED" | grep -qx 'go.mod'; then
  NEW_REQUIRES="$(git diff "$RANGE" -- go.mod 2>/dev/null | awk '/^\+[[:space:]]*[a-zA-Z0-9._\/-]+[[:space:]]+v[0-9]/ && !/^\+\+\+/')"
  if [ -n "$NEW_REQUIRES" ]; then
    ACTIVE_FEATURE="$(grep -m1 '^Active feature:' CLAUDE.md 2>/dev/null | sed -E 's/.*\*\*([^*]+)\*\*.*/\1/' || true)"
    PLAN=""
    if [ -n "$ACTIVE_FEATURE" ]; then
      PLAN="specs/${ACTIVE_FEATURE}/plan.md"
    fi
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      mod="$(printf '%s\n' "$line" | awk '{for(i=1;i<=NF;i++) if($i ~ /\//) {print $i; exit}}')"
      [ -z "$mod" ] && continue
      if [ -z "$PLAN" ] || [ ! -f "$PLAN" ] || ! grep -q -F "$mod" "$PLAN"; then
        VIOLATIONS+=("X: new go.mod dependency \"$mod\" not justified in ${PLAN:-active plan.md} (Principle X requires a justification entry)")
      fi
    done <<< "$NEW_REQUIRES"
  fi
fi

# Rule 3 (Principle I): no CGO in shipped code.
CGO_HITS="$(git diff "$RANGE" -- '*.go' ':!*_test.go' 2>/dev/null | awk '/^\+.*import "C"/ || /^\+[[:space:]]*"C"[[:space:]]*$/')"
if [ -n "$CGO_HITS" ]; then
  VIOLATIONS+=("I: CGO reintroduced (import \"C\") — Principle I forbids CGO in shipped code")
fi

# Rule 4 (Principle VI, gate 5): every exported identifier under
# parse/, model/, render/, findings/ has a godoc comment. Delegated
# to the AST-walking test under tests/coverage/. Runs only when Go
# files in the watched packages are touched in this push, so the
# common case (docs-only or spec-only push) stays cheap.
WATCHED_GO_CHANGED="$(printf '%s\n' "$CHANGED" | grep -E '^(parse|model|render|findings)/.*\.go$' || true)"
if [ -n "$WATCHED_GO_CHANGED" ] && command -v go >/dev/null 2>&1; then
  GODOC_OUT="$(CGO_ENABLED=0 go test ./tests/coverage/... -run TestGodocCoverage -count=1 2>&1)"
  GODOC_RC=$?
  if [ "$GODOC_RC" -ne 0 ]; then
    HAD_HIT=0
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      VIOLATIONS+=("VI: $line")
      HAD_HIT=1
    done < <(printf '%s\n' "$GODOC_OUT" | grep -E ':[0-9]+: (.+: )?(function|method|type|identifier)\b' || true)
    if [ "$HAD_HIT" -eq 0 ]; then
      VIOLATIONS+=("VI: godoc coverage test failed — re-run \`go test ./tests/coverage/... -run TestGodocCoverage -v\` for the missing-godoc list")
    fi
  fi
fi

# Rule 5 (Principle XIV, gate 8): no non-English Latin-script letters
# in checked-in artifacts outside testdata/ and _references/. The
# pattern targets the UTF-8 byte sequences for the Latin-1 Supplement
# letter block (00C0-00FF, with U+00D7 multiplication sign and
# U+00F7 division sign carved out) plus Latin Extended-A (0100-017F).
# Math symbols, em-dashes, arrows, and emoji do NOT match because
# their UTF-8 byte sequences fall outside the C3/C4/C5 leading bytes.
# We force the real GNU grep at /usr/bin/grep when present so an
# interactive shell that aliases grep to ugrep cannot mask the check.
#
# Two existing legitimate uses are exempt by file:line:
#   - specs/003-feedback-backend-worker/spec.md:6 - verbatim
#     user-input quote (Spanish), preserved by the spec exemption
#     ratified at constitution v1.2.0.
#   - feedback-worker/test/validate.test.ts:69-70 - a UTF-8
#     byte-counting test fixture that uses a 2-byte rune by
#     construction.
#
# -I tells grep to skip binary files (PNG, etc.).
ENGLISH_PATTERN='\xC3[\x80-\x96\x98-\xB6\xB8-\xBF]|\xC4[\x80-\xBF]|\xC5[\x80-\xBF]'
# Choose a PCRE-capable grep. /usr/bin/grep is GNU on Linux but BSD on
# macOS, and BSD grep silently exits non-zero on -P (which `|| true`
# below would mask, no-oping the entire gate). Probe `--version` for
# the GNU banner first, then fall back to a feature-test on `grep -P`
# against any grep on PATH (which catches Homebrew's ggrep installed
# as `grep` via a shim, plus Linux distros where /usr/bin/grep is
# missing). If no PCRE-capable grep is reachable, fail closed so the
# operator knows the gate is unenforceable on this host instead of
# silently passing.
if [ -x /usr/bin/grep ] && /usr/bin/grep --version 2>/dev/null | head -1 | grep -q GNU; then
  GREP_BIN=/usr/bin/grep
elif printf '' | grep -P '' >/dev/null 2>&1; then
  GREP_BIN=grep
else
  printf 'pre-push-constitution-guard: PCRE-capable grep not found (need GNU grep with -P or `ggrep`); install via `brew install grep` and shim into PATH, or run on a host with GNU grep. Aborting so gate 8 is not silently bypassed.\n' >&2
  exit 1
fi
ENGLISH_HITS=()
while IFS= read -r f; do
  [ -z "$f" ] && continue
  case "$f" in
    testdata/*|_references/*) continue ;;
  esac
  if [ ! -f "$f" ]; then
    continue
  fi
  while IFS= read -r match; do
    [ -z "$match" ] && continue
    line_num="${match%%:*}"
    case "$f:$line_num" in
      specs/003-feedback-backend-worker/spec.md:6) continue ;;
      feedback-worker/test/validate.test.ts:69) continue ;;
      feedback-worker/test/validate.test.ts:70) continue ;;
    esac
    ENGLISH_HITS+=("$f:$line_num")
  done < <("$GREP_BIN" -InP "$ENGLISH_PATTERN" "$f" 2>/dev/null || true)
done < <(printf '%s\n' "$CHANGED")
for hit in "${ENGLISH_HITS[@]}"; do
  VIOLATIONS+=("XIV: non-English Latin-script letter at $hit (Principle XIV requires English-only artifacts; testdata/ and _references/ are exempt; math/typography/emoji pass)")
done

# Rule 6 (Principle XII / Principle I): suspicious build tags on
# non-test .go files. `// +build ...` (legacy syntax) is a smell.
# `//go:build cgo` is a hard block under Principle I (no CGO).
# Any other `//go:build <foo>` warrants reviewer attention because
# Principle XII forbids build tags that diverge runtime behaviour
# between platforms except where genuinely unavoidable
# (path/filepath is the only documented allowlist entry).
TAG_HITS="$(git diff "$RANGE" -- '*.go' ':!*_test.go' 2>/dev/null | awk '
  /^\+\+\+ b\// { file = substr($0, 7); next }
  /^\+\/\/ \+build / { print "P2|XII|" file ": legacy build tag (// +build), use //go:build instead" }
  /^\+\/\/go:build / {
    tag = $0
    sub(/^\+\/\/go:build /, "", tag)
    if (tag ~ /(^|[ |&!()])cgo($|[ |&!()])/) {
      print "P1|I|" file ": //go:build cgo violates Principle I (no CGO in shipped binary)"
    } else {
      print "P2|XII|" file ": //go:build " tag " - Principle XII reviewer attention required (no allowlist match)"
    }
  }
')"
if [ -n "$TAG_HITS" ]; then
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    sev="${line%%|*}"
    rest="${line#*|}"
    principle="${rest%%|*}"
    msg="${rest#*|}"
    VIOLATIONS+=("$sev - $principle: $msg")
  done <<< "$TAG_HITS"
fi

# Rule 7 (Principle IX): no outbound network in the Go release path.
# The feedback-worker (TypeScript) is the only sanctioned outbound
# path; the Go binary stays offline. We grep added lines under
# parse/, model/, render/, findings/, and cmd/ for a `net/http`
# import or net.Dial-style construct. _test.go files are excluded
# because tests may legitimately exercise an httptest server.
NET_HITS="$(git diff "$RANGE" -- 'parse/*.go' 'model/*.go' 'render/*.go' 'findings/*.go' 'cmd/**/*.go' ':!*_test.go' 2>/dev/null | awk '
  /^\+\+\+ b\// { file = substr($0, 7); next }
  /^\+[[:space:]]*"net\/http"/                      { print file ": import \"net/http\" added (block form)" }
  /^\+[[:space:]]*import[[:space:]]+"net\/http"/    { print file ": import \"net/http\" added (single-line form)" }
  /^\+.*net\.Dial[A-Za-z]*\(/                        { print file ": net.Dial* call added" }
  /^\+.*net\.Listen[A-Za-z]*\(/                      { print file ": net.Listen* call added" }
  /^\+.*net\.Lookup[A-Za-z]*\(/                      { print file ": net.Lookup* call added" }
  /^\+.*net\.Resolve[A-Za-z]*Addr\(/                 { print file ": net.Resolve*Addr call added" }
  /^\+.*http\.Get\(/                                  { print file ": http.Get call added" }
  /^\+.*http\.Post\(/                                 { print file ": http.Post call added" }
  /^\+.*http\.NewRequest/                             { print file ": http.NewRequest call added" }
  /^\+.*http\.Client\{/                               { print file ": http.Client constructor added" }
')"
if [ -n "$NET_HITS" ]; then
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    VIOLATIONS+=("P1 - IX: $line (Principle IX forbids network in the Go release path; the feedback-worker is the only outbound path)")
  done <<< "$NET_HITS"
fi

# Rule 8 (Principle II): no writes inside the input-reading path.
# parse/ and cmd/ MUST NOT call os.Create / os.Mkdir(All) / os.Rename
# / os.Remove / os.RemoveAll / os.WriteFile / ioutil.WriteFile.
# Legitimate writes belong under render/ (the user's -out path) or
# os.TempDir(); flag everything else for review. _test.go is
# excluded because tests can write fixtures to t.TempDir().
WRITE_HITS="$(git diff "$RANGE" -- 'parse/*.go' 'cmd/**/*.go' ':!*_test.go' 2>/dev/null | awk '
  /^\+\+\+ b\// { file = substr($0, 7); next }
  /^\+.*os\.Create([^[:alnum:]_]|$)/        { print file ": os.Create" }
  /^\+.*os\.Mkdir([^[:alnum:]_]|$)/         { print file ": os.Mkdir" }
  /^\+.*os\.MkdirAll([^[:alnum:]_]|$)/      { print file ": os.MkdirAll" }
  /^\+.*os\.Rename([^[:alnum:]_]|$)/        { print file ": os.Rename" }
  /^\+.*os\.Remove([^[:alnum:]_]|$)/        { print file ": os.Remove" }
  /^\+.*os\.RemoveAll([^[:alnum:]_]|$)/     { print file ": os.RemoveAll" }
  /^\+.*os\.WriteFile([^[:alnum:]_]|$)/     { print file ": os.WriteFile" }
  /^\+.*ioutil\.WriteFile([^[:alnum:]_]|$)/ { print file ": ioutil.WriteFile" }
')"
if [ -n "$WRITE_HITS" ]; then
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    VIOLATIONS+=("P2 - II: $line (Principle II forbids writes in the input-reading path; legitimate writes belong in render/ under \$TMPDIR or the user's -out path - annotate the line if intentional)")
  done <<< "$WRITE_HITS"
fi

TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

if [ ${#VIOLATIONS[@]} -eq 0 ]; then
  printf '%s %s %s PASS 0_violations\n' "$TS" "$SHA" "$BRANCH" >> "$LOG"
  exit 0
fi

printf '%s %s %s FAIL %d_violations\n' "$TS" "$SHA" "$BRANCH" "${#VIOLATIONS[@]}" >> "$LOG"

REASON_LINES=("Pre-push constitution guard blocked this push. Violations:")
for v in "${VIOLATIONS[@]}"; do
  REASON_LINES+=("  - $v")
done
REASON_LINES+=("")
REASON_LINES+=("Fix the above, or invoke @agent-pre-review-constitution-guard for a full LLM review.")
REASON_LINES+=("To bypass for a WIP backup push, run: git push --no-verify (discouraged).")
REASON="$(printf '%s\n' "${REASON_LINES[@]}")"

if command -v jq >/dev/null 2>&1; then
  jq -n --arg reason "$REASON" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $reason
    }
  }'
else
  escaped="$(printf '%s' "$REASON" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')"
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":%s}}\n' "$escaped"
fi
exit 0
