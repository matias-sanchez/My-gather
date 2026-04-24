---
name: pr-review-fix-my-gather
description: "Triage Codex / Copilot findings on the current My-gather PR, verify each one, apply fixes that respect the 13-principle constitution in .specify/memory/constitution.md, mark resolved threads as resolved on GitHub, clean stale trigger comments, and optionally re-trigger the review cycle. Use in the My-gather repo whenever the user says `/pr-review-fix-my-gather`, `address review findings`, `fix codex comments`, `fix copilot comments`, `resolve PR review`, or is cycling through a codex-review round on the current branch. Prefer this over the generic `/pr-review-fix` when working in this repo — only this variant walks the 13 principles and uses Go-native validation. <example>Context: Codex has posted findings on the current PR. user: \"address the codex findings\" assistant: \"Invoking /pr-review-fix-my-gather to triage each finding, walk the constitution, apply valid fixes, mark the threads resolved, and commit.\"</example> <example>Context: User wants to clear an iteration. user: \"fix review comments and resolve them\" assistant: \"Launching /pr-review-fix-my-gather — will verify each comment against the code, apply only principle-compliant fixes, resolve the threads on GitHub, and push.\"</example> <example>Context: Bilingual. user: \"aplica las correcciones de codex en el PR\" assistant: \"Running /pr-review-fix-my-gather: verify, principle-walk, fix, resolve threads, push.\"</example>"
---

# PR Review Fix (My-gather)

Read Codex / Copilot findings on the current PR, verify each against the code, apply only fixes that satisfy the 13-principle constitution, mark fixed review threads resolved on GitHub, clean stale trigger comments, and optionally re-trigger the review cycle.

This skill is one half of the review loop:

1. **`/pr-review-trigger`** — post review requests.
2. **`/pr-review-fix-my-gather`** (this skill) — verify, principle-walk, fix, resolve threads, commit, clean, re-trigger.

## Scope and goals

- **Shorten the codex-review loop.** Target mean ≤ 2 rounds per PR. Count `codex-review` commits per branch over time; if the trend isn't down, iterate on this skill.
- **Never regress a constitutional principle when fixing a finding.** The constitution at `.specify/memory/constitution.md` is authoritative. Every candidate fix walks it.
- **Preserve the review trail.** Mark each fixed thread **resolved** on GitHub (GraphQL `resolveReviewThread`). Do not bulk-delete review comments to "fake" resolution — deletion destroys audit history; resolution preserves it and signals the loop is converging.

## Tool preferences

- **Preferred**: GitHub MCP tools (`mcp__github__*`) for PR metadata, review comments, issue comments, and review-thread mutations when available.
- **Fallback**: `gh` CLI (verify `gh auth status` first; if not logged in, ask the user to run `! gh auth login`).
- **Bot login identifiers** (used for filtering):
  - Codex: `chatgpt-codex-connector[bot]`
  - Copilot: `Copilot`
  - If this project starts using other reviewers, add their logins to the filter at the top of Step 1.

## Procedure

Run the steps in order. Do not skip.

### Step 1 — Detect the PR and collect findings

```bash
BRANCH=$(git branch --show-current)
PR_NUMBER=$(gh pr view "$BRANCH" --json number --jq '.number')
OWNER=$(gh repo view --json owner --jq '.owner.login')
REPO=$(gh repo view --json name --jq '.name')
```

Collect three things:

1. **Inline review comments** (attached to specific file:line pairs):
   ```bash
   gh api "repos/$OWNER/$REPO/pulls/$PR_NUMBER/comments" \
     --jq '.[] | select(.user.login == "Copilot" or .user.login == "chatgpt-codex-connector[bot]") | {id, path, line: (.line // .original_line), body}'
   ```
2. **Issue-level comments** from the same bots (often contain summary findings):
   ```bash
   gh api "repos/$OWNER/$REPO/issues/$PR_NUMBER/comments" \
     --jq '.[] | select(.user.login == "Copilot" or .user.login == "chatgpt-codex-connector[bot]") | {id, body}'
   ```
3. **Review thread IDs** (needed to mark resolved in Step 8):
   ```bash
   gh api graphql -f query='
     query($owner:String!,$name:String!,$number:Int!){
       repository(owner:$owner,name:$name){
         pullRequest(number:$number){
           reviewThreads(first:100){ nodes{ id isResolved comments(first:1){ nodes{ databaseId author{ login } path originalLine } } } }
         }
       }
     }' -f owner="$OWNER" -f name="$REPO" -F number="$PR_NUMBER" \
     --jq '.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false)'
   ```
   Record a map `{comment_database_id → thread_id}` so fixing comment `X` maps to resolving thread `Y`.

### Step 2 — Verify each finding against the actual code

For every finding:

1. **Read the referenced file** at the exact line using the Read tool. Do not trust the reviewer's quoted context — files drift.
2. **Trace the logic.** Is the bug real? Does the exception actually need widening? Is the suggested behavior change justified by reading neighboring code?
3. **Grep for usages** when a finding claims "caller X still depends on Y". Use Grep against the current working tree, not the reviewer's snapshot.

Verification discipline is strict: **a finding is not valid until you have re-read the code that proves it.**

### Step 3 — Walk the constitution on the proposed fix (gate)

For each **valid** finding, before applying a fix, answer the questions below against **the fix the reviewer is proposing**. If any answer is yes, the suggested remedy violates the constitution. Do not apply it — either redesign the fix yourself or dismiss.

Re-read `.specify/memory/constitution.md` at the start of each run; the version may have changed.

| Principle | Gate question — answer yes = don't apply this fix as proposed |
|---|---|
| I. Single Static Binary | Does the fix require CGO, dynamic linking, or a runtime install step? |
| II. Read-Only Inputs | Does the fix write, rename, or create files inside the pt-stalk input tree? |
| III. Graceful Degradation | Does the fix panic on malformed input, drop data silently, or replace a structured diagnostic with a log line / comment? |
| IV. Deterministic Output | Does the fix introduce map iteration (without sorted keys), unstable sort, `time.Now()` reaching the render path, locale-dependent formatting, unformatted floats, goroutine-scheduled work whose order affects output, or random IDs without a fixed seed? |
| V. Self-Contained HTML Reports | Does the fix add a CDN link, external font, analytics endpoint, runtime fetch, or any asset not routed through `//go:embed`? |
| VI. Library-First Architecture | Does the fix have `parse/` / `model/` / `render/` read CLI flags, write to `os.Stdout` / `os.Stderr`, or depend on `package main`? Does it add or change an exported identifier without a godoc comment? |
| VII. Typed Errors | Does the fix use `fmt.Errorf(...)` for a branchable condition, or drop `%w` when an underlying typed error exists? |
| VIII. Reference Fixtures & Golden Tests | Does the fix add or change a parser without also adding/updating the matching fixture under `testdata/` and golden under `testdata/golden/`? |
| IX. Zero Network at Runtime | Does the fix introduce `net/http` client code, DNS lookup, or any outbound I/O in release-path files? |
| X. Minimal Dependencies | Does the fix add a `go.mod require` entry without a justification line in the active feature's `plan.md`? |
| XI. Reports Optimized for Humans Under Pressure | Does the fix promote an exhaustive dump above the "what triggered / state at trigger / deltas" spine? |
| XII. Pinned Go Version | Does the fix change `go.mod`'s `go` directive as a side effect of another change? |
| **XIII. Canonical Code Path (NON-NEGOTIABLE)** | Does the fix leave an old function, type, or code path alongside a replacement? Add a fallback (`try A, on error silently try B`)? Introduce an `if useNew { ... } else { ... }` internal flag? Keep a re-export or alias after a rename? Preserve a compat shim for internal callers? |

Any yes means: **do not apply this fix as proposed**. Your options are:

- **Alternative remedy** — the underlying finding is valid, the reviewer's fix is wrong. Implement a principle-compliant fix yourself, then reply to the thread with a one-line note explaining the alternative (`Applied alternative fix: <what you did> — reviewer's proposed remedy would have violated Principle <roman>`), and mark the thread resolved in Step 8.
- **Dismiss** — the finding itself is invalid *and* the fix would violate a principle. Reply `Dismissed: conflicts with Principle <roman> (<one-line reason>)`, resolve the thread, move on.

**Crucial distinction.** A finding that *itself* cites a constitutional violation (e.g. "this ranges over a map which breaks Principle IV") is **valid and must be fixed** — the gate is about **the fix**, not the finding. Always read the finding's *claim* first, then evaluate the *proposed remedy*.

### Step 4 — Classify each finding

For each finding write a single line in your working plan:

- `FIX: <path>:<line> — <one-line description>` — valid finding, proposed remedy is principle-clean
- `ALT: <path>:<line> — <one-line description> — alternative: <what you'll do instead>` — valid finding, your remedy differs
- `DISMISS: <path>:<line> — <one-line reason citing principle or false-positive pattern>`

### Step 5 — Apply fixes

For each `FIX` and `ALT`:

1. Edit the file with the Edit tool.
2. **If the change touches `parse/*.go`** (Principle VIII) — add or update the fixture in `testdata/` and the golden in `testdata/golden/` in the same change. Regenerating goldens goes through `go test ./parse/... -update`; review the diff before staging. Never run `-update` across the whole repo silently.
3. **If the change touches `render/*.go`** — re-read the edited code for map iteration, non-stable sort, `time.Now()`, unformatted floats. These are the highest-rate Principle IV regressions.
4. **If the change introduces or renames an exported identifier** — add a godoc comment (Principle VI).
5. Run the narrow test for the touched package before moving on:
   ```bash
   go test ./<touched-package>/... -count=1
   ```

### Step 6 — Full local validation

Match the CI gates and the 7 merge gates from `Development Workflow & Quality Gates`:

```bash
go vet ./...
go test ./... -count=1
```

If CI has additional targets (determinism diff, cross-compile, lint), run their `make` equivalents when available (`make determinism`, `make lint`, etc.) — check the `Makefile` at the repo root for the current target names. Do not fabricate target names.

Then invoke the **`pre-review-constitution-guard`** subagent to catch anything you missed:

> `@agent-pre-review-constitution-guard`

If the guard returns `FIX P1s FIRST` or `FIX P1s + REVIEW P2s`, loop back to Step 5 and address the P1s before continuing.

### Step 7 — Commit and push

```bash
git add <specific paths — do NOT use git add -A>
git commit -m "$(cat <<'EOF'
pr-review: address codex/copilot round <N> (<M> fixed, <K> dismissed)

Fixed:
  - <path>:<line> — <one-line>
  - ...

Dismissed:
  - <path>:<line> — <one-line reason, citing principle when applicable>
  - ...

EOF
)"
git push
```

The pre-push hook (`scripts/hooks/pre-push-constitution-guard.sh`) runs automatically and will block the push if any of its mechanical rules trip. That's expected — fix the flagged issue and re-push. Do **not** bypass with `--no-verify` to "make the push go through"; the hook is there because bypassing it is how the codex-review loop ballooned to 8 rounds in the first place.

### Step 8 — Mark fixed review threads resolved (GitHub)

For each finding you **fixed** or applied an **alternative remedy** to, mark its review thread resolved so the PR shows convergence progress:

```bash
# Resolve one thread by its GraphQL node ID (from Step 1's thread map)
gh api graphql -f query='
  mutation($id:ID!){
    resolveReviewThread(input:{threadId:$id}){ thread{ id isResolved } }
  }' -f id="$THREAD_ID"
```

If you dismissed the finding (Step 3 / Step 4), also reply on the thread with the one-line dismissal reason **before** resolving it, so the audit trail shows why:

```bash
gh api "repos/$OWNER/$REPO/pulls/$PR_NUMBER/comments/$COMMENT_ID/replies" \
  -f body="Dismissed: conflicts with Principle XIII (would keep duplicate code path). No action."
# then resolve the thread with the GraphQL mutation above
```

After processing every finding, verify:

```bash
gh api graphql -f query='
  query($owner:String!,$name:String!,$number:Int!){
    repository(owner:$owner,name:$name){
      pullRequest(number:$number){
        reviewThreads(first:100){ nodes{ isResolved } }
      }
    }
  }' -f owner="$OWNER" -f name="$REPO" -F number="$PR_NUMBER" \
  --jq '[.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false)] | length'
```

This should print `0` if you've processed every bot-posted thread. If higher, either you missed a finding or a human reviewer has an unresolved thread — do not resolve human threads automatically; surface the count to the user.

### Step 9 — Clean stale trigger comments (optional, opt-in)

**Separate** from resolving. Delete only your prior `@codex review` / `@copilot review` trigger issue-comments so the next cycle's trigger stands alone. Do **not** bulk-delete bot review comments — the resolved-thread convention preserves them collapsed, which is what the reviewer wants.

```bash
gh api "repos/$OWNER/$REPO/issues/$PR_NUMBER/comments" \
  --jq '.[] | select((.body | contains("@codex review")) or (.body | contains("@copilot review"))) | .id' \
  | while read -r id; do
      gh api -X DELETE "repos/$OWNER/$REPO/issues/comments/$id"
    done
```

Only run Step 9 if the user asked for a clean trigger lead-in, or if Step 10 will re-trigger and the old trigger would otherwise confuse the next review cycle.

### Step 10 — Decide next action

- **Fixes were applied.** Tell the user: count of fixes, count of dismissals, count of threads resolved, count of threads still open (should be 0 for bots). Ask whether to run `/pr-review-trigger` for another round.
- **Only dismissals.** Tell the user every finding was either a false positive or a principle-conflicting suggestion. Threads are resolved with dismissal notes. Ask whether to re-trigger anyway.
- **No findings at all.** The PR is review-clean. Suggest merging.

## Non-negotiable rules

- **Always verify findings against live code.** No fix applied without a file Read confirming the claim.
- **Always walk the principle gate before editing.** A green gate is a precondition for every Edit.
- **Never apply a fix that would violate Principle XIII.** No duplicate code paths, no silent fallbacks, no post-rename shims. Prefer alternative remedy over the reviewer's suggestion when the reviewer asks to keep old code "just in case".
- **Never bypass the pre-push hook** with `--no-verify`. If the hook blocks, the code isn't ready.
- **Never bulk-delete review comments** to fake convergence. Resolve threads via GraphQL; delete only your own trigger comments (Step 9).
- **Never touch human review threads** automatically. Only resolve threads whose top comment author is `Copilot` or `chatgpt-codex-connector[bot]`.
- **Never mix commits.** Fixes for review findings go in one commit. Do not bundle unrelated refactors — that violates XII's "upgrades are their own commit" spirit and makes the review trail unreadable.

## My-gather-specific false-positive patterns

Dismiss these when codex / copilot proposes them. Cite the principle in the dismissal reply.

1. **"Use goroutines to speed up parsing."** → Dismiss. Principle IV — any parallelism in `parse/` / `model/` / `render/` introduces non-determinism risk for marginal gain.
2. **"Parallelize golden generation."** → Dismiss. Same as above + Principle VIII (goldens are a reviewed step, not a perf target).
3. **"Fall back to raw text when the structured parser fails."** → Dismiss. Principles III + XIII — attach a diagnostic and surface the failure; do not silently fall through.
4. **"Load chart.js from a CDN."** → Dismiss. Principles V + IX.
5. **"Use `map[string]string` for this lookup — iteration order doesn't matter."** → Dismiss. Principle IV — no Go map is iteration-deterministic; if the data feeds render, sort keys before ranging.
6. **"Keep `parseFoo` alongside the new `parseFooV2` for compatibility."** → Dismiss. Principle XIII — canonical path only. Delete `parseFoo` in the same change.
7. **"Re-export the renamed identifier so old callers keep working."** → Dismiss. Principle XIII — update all call sites in the same commit.
8. **"Mock the filesystem for this parser test."** → Dismiss. Principle VIII — use a real fixture from `_references/examples/`.
9. **"Catch `error` broadly here to avoid propagating failures."** → Dismiss. Principles III + VII — surface as typed error or structured diagnostic.
10. **"Add a retry loop around this operation."** → Dismiss unless the operation is a boundary I/O (the tool is offline-first and deterministic — retries hide real failures).
11. **"Bump Go to the latest tip as part of this fix."** → Dismiss. Principle XII — Go upgrades are their own reviewed commit.
12. **"Add a pre-existing-pattern refactor."** → Dismiss. Out of scope for a review-fix cycle; propose it as a separate issue if worth doing.

## Convergence metric

After each run, append one line to `.claude/pre-review.log` (already gitignored):

```
<ts> <pr#> <round#> fixed=<M> dismissed=<K> resolved=<R>
```

Use this to track whether the loop is shrinking. Target: mean rounds per PR ≤ 2 within two weeks of adopting this skill + the pre-review agent + the pre-push hook.
