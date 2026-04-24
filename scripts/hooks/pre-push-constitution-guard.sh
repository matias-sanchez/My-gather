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

# Rule 1 (Principle VIII): a new parser file under parse/ MUST ship with
# a fixture and a golden in the same push.
NEW_PARSERS="$(git diff --name-only --diff-filter=A "$RANGE" -- 'parse/*.go' ':!parse/*_test.go' 2>/dev/null || true)"
for f in $NEW_PARSERS; do
  base="$(basename "$f" .go)"
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
