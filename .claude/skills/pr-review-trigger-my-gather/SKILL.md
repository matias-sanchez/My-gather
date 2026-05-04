---
name: pr-review-trigger-my-gather
description: "Start a fresh Codex + Copilot review cycle on the current My-gather PR. Runs a local constitution pre-flight first (so external bots do not re-find what the local guard already catches), gathers Go-native context, deletes only the prior trigger issue-comments (leaving resolved bot review threads intact as audit trail), and posts focused trigger comments that cite the constitution and the My-gather-specific false-positive patterns. Use in the My-gather repo whenever the user says `/pr-review-trigger-my-gather`, `trigger review`, `ask codex to review`, `start a review round`, `request PR review`, or wants the external reviewers to look at the current state of the PR. Prefer this over the generic `/pr-review-trigger` when working in this repo — only this variant pre-flights with the constitution guard, preserves resolved threads from prior cycles, and ships the repo-specific rule context to the bots. <example>Context: User just pushed fixes and wants another review round. user: \"trigger review\" assistant: \"Invoking /pr-review-trigger-my-gather — will pre-flight with the constitution guard, then post Codex + Copilot triggers with My-gather rule context.\"</example> <example>Context: User wants to kick off the first review on a new PR. user: \"ask codex to review this PR\" assistant: \"Launching /pr-review-trigger-my-gather: guard pre-flight, Go-native context, post triggers that cite the constitution.\"</example> <example>Context: After /pr-review-fix-my-gather finished a round, user wants to start the next cycle. user: \"start another review cycle\" assistant: \"Running /pr-review-trigger-my-gather to pre-flight locally and post new Codex + Copilot requests without deleting the resolved threads from the prior round.\"</example>"
---

# PR Review Trigger (My-gather)

Start a fresh automated review cycle on the current branch's PR by requesting reviews from Codex and Copilot, anchored to the My-gather constitution and Go toolchain.

This skill is one half of the review loop:

1. **`/pr-review-trigger-my-gather`** (this skill) — pre-flight, clean stale trigger comments, post trigger requests.
2. **`/pr-review-fix-my-gather`** — verify, principle-walk, fix, resolve threads, commit, clean, optionally re-trigger.

## Scope and goals

- **Make the external review round count.** Every bot round costs latency and time. Running a local constitution pre-flight first means bots look at code that already passes the guard — so their findings are genuine signal, not a replay of what we already know.
- **Give the bots the right ruleset.** Generic reviewers suggest patterns that this repo forbids (goroutines in `parse/`, CDN fetches, silent fallback parsers, re-exports after rename, new unjustified deps). The trigger comment cites the constitution and lists the common false-positive patterns up front, so bots can self-filter.
- **Preserve the review trail.** The companion fix skill resolves threads via GitHub GraphQL; this skill MUST NOT delete those resolved bot threads. It cleans only the prior `@codex review` / `@copilot review` trigger issue-comments.

## Tool preferences

- **Preferred**: GitHub MCP tools (`mcp__github__*`) for PR metadata and comment posting when available.
- **Fallback**: `gh` CLI (verify `gh auth status` first; if not logged in, ask the user to run `! gh auth login`).
- **Bot login identifiers** (used for filtering):
  - Codex: `chatgpt-codex-connector[bot]`
  - Copilot: `Copilot`

## Procedure

Run the steps in order. Do not skip.

### Step 0 — Local constitution pre-flight

Before spending external bot cycles, ask the local guard:

> `@agent-pre-review-constitution-guard`

Read the verdict on the last line of the guard's report.

- **`READY TO PUSH`** — proceed to Step 1.
- **`FIX P1s FIRST`** — stop. Report the P1s to the user. Do not post external triggers. Triggering with known-broken P1s just asks bots to re-find them, which is the exact loop this skill is built to shorten.
- **`FIX P1s + REVIEW P2s`** — stop for the same reason, then ask the user whether to proceed anyway (sometimes a P2 is acceptable mid-iteration).

This step is non-negotiable except when the user explicitly says "trigger anyway" or "bypass guard" — record that choice in the trigger comment itself ("Guard bypassed: <reason>") so reviewers know the local check did not pass.

### Step 1 — Detect PR

```bash
BRANCH=$(git branch --show-current)
PR_NUMBER=$(gh pr view "$BRANCH" --json number --jq '.number' 2>/dev/null)
BASE_REF=$(gh pr view "$BRANCH" --json baseRefName --jq '.baseRefName' 2>/dev/null)
HEAD_SHA=$(gh pr view "$BRANCH" --json headRefOid --jq '.headRefOid' 2>/dev/null)
OWNER=$(gh repo view --json owner --jq '.owner.login')
REPO=$(gh repo view --json name --jq '.name')
BASE_COMPARE_REF="origin/$BASE_REF"
git rev-parse --verify "$BASE_COMPARE_REF" >/dev/null 2>&1 || BASE_COMPARE_REF="$BASE_REF"
```

If no PR exists, tell the user and stop. Do not open one silently.

### Step 2 — Gather context (Go-native)

Run these to build the trigger comment body. Cache each variable:

```bash
# Commit and diff scope
COMMIT_COUNT=$(git log --oneline "$BASE_COMPARE_REF".."$BRANCH" | wc -l | tr -d ' ')
DIFF_STAT=$(git diff --shortstat "$BASE_COMPARE_REF"..."$BRANCH")     # e.g. "12 files changed, 340 insertions(+), 45 deletions(-)"

# Go static analysis — one line summary
VET_STATUS=$(go vet ./... 2>&1 | head -1)
[ -z "$VET_STATUS" ] && VET_STATUS="go vet clean"

# Go tests — pass/fail summary
TEST_OUT=$(go test ./... -count=1 2>&1)
TEST_STATUS=$(printf '%s\n' "$TEST_OUT" | awk '
  /^ok/      { ok++ }
  /^FAIL/    { fail++ }
  /^---/     { }
  END { printf("%d packages ok", ok); if (fail > 0) printf(", %d FAIL", fail) }')

# Constitution version (so bots anchor to the right ruleset)
CONSTITUTION_VERSION=$(grep -m1 '^\*\*Version\*\*' .specify/memory/constitution.md | sed -E 's/.*Version\*\*:[[:space:]]*([0-9.]+).*/\1/')
```

If `go vet` or `go test` fails, stop. Triggering bots on a broken working tree wastes both of you. Report the failure to the user and let them decide.

### Step 3 — Clean only stale trigger issue-comments

**Do not delete bot review comments or resolved review threads.** The fix skill resolves threads via GraphQL precisely to preserve the audit trail — deleting them here would erase proof the prior round converged.

Delete only the prior `@codex review` / `@copilot review` trigger issue-comments so the new trigger stands alone:

```bash
gh api "repos/$OWNER/$REPO/issues/$PR_NUMBER/comments" \
  --jq '.[] | select((.body | contains("@codex review")) or (.body | contains("@copilot review"))) | .id' \
  | while read -r id; do
      gh api -X DELETE "repos/$OWNER/$REPO/issues/comments/$id"
    done
```

Verify:

```bash
REMAINING=$(gh api "repos/$OWNER/$REPO/issues/$PR_NUMBER/comments" \
  --jq '[.[] | select((.body | contains("@codex review")) or (.body | contains("@copilot review")))] | length')
[ "$REMAINING" -eq 0 ] || { echo "Stale trigger comments remain ($REMAINING). Investigate before posting new ones."; exit 1; }
```

If the user explicitly asked for a full wipe of *all* bot review comments (for example, they want to start a PR over from scratch), expand cleanup — but say clearly in the reply to the user which mode you used, and remind them the resolved-thread trail is gone.

### Step 4 — Post the trigger comments

Post TWO separate issue comments — one addressed to Codex, one to Copilot. They share the same body, since the rules they should apply are the same.

Use this exact template. Substitute `${VAR}` placeholders with values from Step 2. Commit SHA is shortened to 12 chars.

```text
@codex review

Code review requested on `${BRANCH}` — ${COMMIT_COUNT} commits, ${DIFF_STAT}, latest commit `${HEAD_SHA:0:12}`. Local status: ${VET_STATUS}, tests: ${TEST_STATUS}.

**Ruleset:** this repo is governed by `.specify/memory/constitution.md` (version ${CONSTITUTION_VERSION}). Please read it before reviewing — it defines what is and is not a valid fix here. In particular:

- **Principle IV (Deterministic Output)**: output MUST be byte-identical across runs. No map iteration without sorted keys, no unstable sort, no `time.Now()` reaching the render path, no locale-dependent float formatting, no random IDs.
- **Principle VIII (Reference Fixtures & Golden Tests)**: new parsers MUST ship with a fixture under `testdata/` and a golden under `testdata/golden/` in the same change.
- **Principle XIII (Canonical Code Path, NON-NEGOTIABLE)**: exactly one canonical implementation path per behaviour, workflow, API, helper, worker route, UI behaviour, review skill, or automation path. No duplicated code, no hidden internal fallbacks (`try A, on error try B`), no post-rename compatibility shims, no `if useNew { ... } else { ... }` internal flags. When a path is replaced, the old one is deleted in the same change. External degradation must be observable, tested or explicitly reviewed, and routed through the canonical owner.
- **Principle XIV (English-Only Durable Artifacts)**: all checked-in code, comments, commit messages, docs, and configuration MUST be English. The only exempt content is under `testdata/` and `_references/` (raw pt-stalk input).
- **Principle XV (Bounded Source File Size)**: governed first-party source-code files MUST stay at or below 1000 lines. Specs, docs, fixtures, goldens, lockfiles, and vendored/minified third-party assets are outside this source-code rule.

**Focus:** real bugs, correctness, determinism regressions, exception handling, behavioural drift from deleted code, principle violations.

**Do NOT suggest any of the following** — these are known anti-patterns for this repo and will be dismissed:

- Keeping legacy/fallback code or dual implementations (Principle XIII).
- Re-exports or compatibility aliases after a rename (Principle XIII).
- External degradation that is hidden, untested, or routed around the canonical owner (Principle XIII).
- Goroutines, parallelism, or concurrent parsing anywhere in `parse/`, `model/`, or `render/` (Principle IV).
- External fetches, CDN links, or runtime network I/O (Principles V + IX).
- Silent fallback parsers — always attach a structured diagnostic instead (Principle III).
- Raw map iteration in anything that feeds `render/` — sort keys first (Principle IV).
- New `go.mod require` entries without a justification line in `specs/<active-feature>/plan.md` (Principle X).
- CGO, `import "C"`, or any dynamic-link assumption (Principle I).
- `fmt.Errorf` for branchable conditions — use typed errors with `%w` (Principle VII).
- Bumping the Go directive in `go.mod` as a side effect (Principle XII).
- Non-English content in code, comments, commit messages, docs, or configuration outside `testdata/` and `_references/` (Principle XIV).
- Letting a governed first-party source-code file exceed 1000 lines (Principle XV).
- Style, formatting, or import ordering — CI and `gofmt` already enforce those.

**Do NOT implement or push changes.** Review-only. Findings as inline comments, please.
```

Second comment is identical except the first line is `@copilot review`.

### Step 5 — Report

Tell the user, in this order:

1. Guard verdict from Step 0 (and whether it passed).
2. Context captured in Step 2 (commits, diff stat, vet/test status).
3. Count of prior trigger comments deleted in Step 3.
4. Confirmation both triggers posted, with their comment IDs for traceability.
5. Next step: wait for Codex + Copilot to respond, then call `/pr-review-fix-my-gather` to process findings.

## Non-negotiable rules

- **Never skip the guard pre-flight** unless the user explicitly waives it. Record the waiver reason in the trigger comment so reviewers know local checks did not pass.
- **Never delete resolved bot review threads.** They are the convergence audit trail. Only prior trigger issue-comments are fair game for automatic cleanup.
- **Never trigger on a broken working tree.** If `go vet` or `go test` fails in Step 2, stop and report — do not spend bot cycles on code that does not build.
- **Never post more than one Codex trigger and one Copilot trigger per round.** Bots see duplicate mentions and occasionally respond twice.
- **Never mix language in the trigger body.** This repo is English-only for artifacts (see CLAUDE.md language convention); the trigger comment is an artifact.
- **Never open a PR implicitly** if one does not exist. Ask the user to open it explicitly so the branch-to-PR mapping is their choice.

## Convergence metric

Append one line per run to `.claude/pre-review.log` (already gitignored):

```
<ts> <pr#> trigger round=<N> guard=<PASS|FAIL> cleaned=<C> posted=2
```

Together with the `fixed=` / `dismissed=` / `resolved=` lines emitted by `/pr-review-fix-my-gather`, this lets you count the round-trip cost of the loop over time. Target: mean external rounds per PR ≤ 2 within two weeks of adopting both skills + the guard agent + the pre-push hook.
