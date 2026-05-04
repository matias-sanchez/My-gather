---
name: pr-review-loop-my-gather
description: "Drive the current My-gather PR to green by looping `/pr-review-fix-my-gather` → `/pr-review-trigger-my-gather` until Codex returns zero findings. One-shot invocation: the skill self-schedules the next iteration via ScheduleWakeup, resolves each batch of threads, and exits only when Codex confirms the PR is clean or a safety rail fires. Use whenever the user says `/pr-review-loop-my-gather`, `loop the reviews`, `drive the PR to green`, `iterate reviews until clean`, `keep fixing until Codex is happy`, or wants unattended convergence of the Codex + Copilot review cycle on the current PR. <example>Context: User wants the PR driven to merge-ready without babysitting each round. user: \"drive the PR to green\" assistant: \"Invoking /pr-review-loop-my-gather — I will fix the current findings, resolve the threads, trigger another round, and wake myself every ten minutes to check Codex until there are zero findings or a safety rail fires.\"</example> <example>Context: User just pushed a big change and wants the loop to chase tail-end bugs. user: \"iterate on code reviews until there are no more findings\" assistant: \"Running /pr-review-loop-my-gather with the default six-iteration cap; will stop as soon as Codex returns zero findings or progress stalls.\"</example> <example>Context: User wants the loop on a specific cadence. user: \"keep looping on the PR until Codex is happy, check every ten minutes\" assistant: \"Launching /pr-review-loop-my-gather with a ten-minute wake cadence between iterations.\"</example>"
---

# PR Review Loop (My-gather)

Drive the current branch's PR to a green Codex review by looping the fix → trigger cycle until Codex reports zero findings, then stop. One-shot invocation — the skill self-schedules the next iteration between rounds; the user never has to re-invoke.

This skill is the third leg of the review workflow:

1. `/pr-review-trigger-my-gather` — one-shot: pre-flight, then post Codex + Copilot triggers.
2. `/pr-review-fix-my-gather` — one-shot: verify, principle-walk, fix, resolve threads, push.
3. `/pr-review-loop-my-gather` (this skill) — composes (1) and (2) in a bounded loop until convergence.

## What "green" means here

**Green = the most recent Codex review on the PR contains zero open findings.**

Operationally, after running `/pr-review-fix-my-gather` and `/pr-review-trigger-my-gather`, the loop waits for Codex to respond and then counts open review threads authored by `chatgpt-codex-connector[bot]`. If that count is zero, the PR is green and the loop exits with `CONVERGED`. Copilot findings are addressed by the fix skill but do not define the stop condition — Codex is the authoritative reviewer for this gate.

## Scope and non-negotiables

- **Composes, never duplicates.** This skill calls the other two via their slugs — it does not reimplement the fix or trigger logic. Any change to fix or trigger semantics happens in those files (Principle XIII: canonical code path, no duplicated implementations).
- **Every applied fix inherits the full 15-principle walk** from `/pr-review-fix-my-gather`. That walk is **not** bypassed, shortened, or softened inside this loop. If a proposed fix would violate a principle, the fix skill dismisses it or applies an alternative remedy — same as a single-shot run.
- **No fix ever introduces a duplicated or fallback implementation** (Principle XIII). If the fix skill returns without applying a change for a given finding because the only remedy would violate XIII, that finding is dismissed with a reply citing XIII — the loop does not retry it with a looser remedy.
- **English-only** (Principle XIV) across every artifact this skill touches: commit messages, review replies, thread-resolution notes, and log lines.
- **Commit + push is the default flow** — inherited from `/pr-review-fix-my-gather`'s Step 7. This skill never modifies code directly; code changes only come through the fix skill.

## Inputs

Accepted via the skill argument string when invoked; all optional.

- `maxIterations=N` — hard cap on how many fix+trigger rounds to run. Default **6**. Safety rail: prevents an infinite loop if Codex finds new issues every round.
- `waitSeconds=N` — seconds to sleep between "triggered" and "check Codex response". Default **600** (10 minutes). Minimum 270 (keeps prompt cache warm); maximum 3600.
- `maxWaitPerRoundSeconds=N` — total time to wait for Codex to respond in one round before timing out. Default **2400** (40 minutes). If Codex has not posted a new review within this window, the loop exits with `TIMEOUT`.

Example: `/pr-review-loop-my-gather maxIterations=4 waitSeconds=600`

## Procedure

Each invocation executes exactly one of: (a) the first iteration of a fresh loop, or (b) a scheduled continuation of an in-flight loop. State is carried between wake-ups via `.claude/pre-review.log` and the PR itself (Codex review timestamps, resolved-thread count).

### Step 0 — Load state and budget

- Parse the argument string for `maxIterations`, `waitSeconds`, `maxWaitPerRoundSeconds`. Apply defaults for any missing field.
- Count prior loop entries from the current branch in `.claude/pre-review.log` (lines containing `loop branch=<BRANCH>`). That count is the current iteration number `N` (0-indexed).
- If `N >= maxIterations`, exit with `MAX_ITERATIONS` (see Step 6).

### Step 1 — Detect PR and current Codex status

```bash
BRANCH=$(git branch --show-current)
PR_NUMBER=$(gh pr view "$BRANCH" --json number --jq '.number')
OWNER=$(gh repo view --json owner --jq '.owner.login')
REPO=$(gh repo view --json name --jq '.name')
```

Count open Codex review threads (threads whose top comment is authored by `chatgpt-codex-connector[bot]` and whose `isResolved` is false):

```bash
OPEN_CODEX=$(gh api graphql -f query='
  query($owner:String!,$name:String!,$number:Int!){
    repository(owner:$owner,name:$name){
      pullRequest(number:$number){
        reviewThreads(first:100){ nodes{ isResolved comments(first:1){ nodes{ author{ login } } } } }
      }
    }
  }' -f owner="$OWNER" -f name="$REPO" -F number="$PR_NUMBER" \
  --jq '[.data.repository.pullRequest.reviewThreads.nodes[]
         | select(.isResolved == false)
         | select(.comments.nodes[0].author.login == "chatgpt-codex-connector[bot]")] | length')
```

Also record the SHA of the most recent Codex review so the wait-for-response step in Step 5 can detect "Codex has responded again":

```bash
LAST_CODEX_REVIEW_ID=$(gh api "repos/$OWNER/$REPO/pulls/$PR_NUMBER/reviews" \
  --jq '[.[] | select(.user.login == "chatgpt-codex-connector[bot]")] | last | .id // empty')
```

### Step 2 — Convergence check (the exit condition)

If `OPEN_CODEX == 0`:

- The PR is green by Codex.
- Emit the convergence marker and stop. Do **not** trigger another round — that would be wasted API time and, if it surfaced a new finding, would reopen the loop unnecessarily.
- Write `<ts> loop branch=<BRANCH> pr=<PR_NUMBER> iter=<N> status=CONVERGED open_codex=0` to `.claude/pre-review.log`.
- Tell the user: "PR #<N> is review-clean on Codex (0 open threads). Ready to merge. Loop exited CONVERGED after <N> iterations."
- Return. No further scheduling.

If `OPEN_CODEX > 0`: proceed.

### Step 3 — Invoke `/pr-review-fix-my-gather`

Call the fix skill:

> `/pr-review-fix-my-gather`

The fix skill runs its full procedure: verify each finding, walk all 15 principles, apply fixes (or alternative remedies, or dismissals), run `go vet ./...` and `go test ./...`, invoke `@agent-pre-review-constitution-guard`, commit, push, and resolve each processed thread via GraphQL.

**Critical contract for this loop**: if the fix skill dismisses a finding because the only remedy would violate a principle (most commonly XIII — duplicated code / silent fallback — or XIV — non-English content), that dismissal is **final**. The loop MUST NOT retry that finding on the next iteration. Because the fix skill resolves the thread on dismissal, Codex's next review will not re-raise a thread it already resolved — but if Codex does raise it again anyway, the fix skill will dismiss it again with the same XIII/XIV citation. That is correct behaviour; not a bug to work around.

**Non-progress detection**: capture the number of threads the fix skill actually changed this round. The fix skill appends a line like `<ts> <pr#> <round#> fixed=<M> dismissed=<K> resolved=<R>` to `.claude/pre-review.log`. If `fixed + dismissed == 0`, the fix skill did no work — Codex's findings are all stale or already-resolved, and continuing is pointless. Exit with `NO_PROGRESS` (see Step 6).

### Step 4 — Invoke `/pr-review-trigger-my-gather`

Call the trigger skill:

> `/pr-review-trigger-my-gather`

The trigger skill runs its pre-flight with `@agent-pre-review-constitution-guard`. If the guard returns anything other than `READY TO PUSH`, the trigger skill stops. In that case the loop MUST also stop — triggering with known-broken P1s would ask Codex to re-find what the guard already caught. Exit with `GUARD_FAIL` (see Step 6).

If trigger succeeded, two new `@codex review` / `@copilot review` issue comments are now on the PR.

### Step 5 — Actively wait for the next Codex response

Write an iteration line to the log:

```
<ts> loop branch=<BRANCH> pr=<PR_NUMBER> iter=<N> status=WAITING open_codex_before=<OPEN_CODEX_BEFORE> waiting_from_review_id=<LAST_CODEX_REVIEW_ID>
```

Call `ScheduleWakeup` with `delaySeconds=$waitSeconds` (default 600) and `prompt="/pr-review-loop-my-gather maxIterations=$maxIterations waitSeconds=$waitSeconds maxWaitPerRoundSeconds=$maxWaitPerRoundSeconds"`. The `reason` passed to ScheduleWakeup names the PR and iteration so the user sees the state in telemetry.

When the wake-up fires, the next invocation begins at Step 0 again. It will:

- See that Codex has (or has not) posted a newer review than `LAST_CODEX_REVIEW_ID`.
- If a newer review exists, proceed to Step 1 of the new iteration (count open threads, branch on convergence).
- If no newer review exists AND the cumulative wait across scheduled wake-ups on this iteration has exceeded `maxWaitPerRoundSeconds`, exit with `TIMEOUT` (see Step 6). Cumulative wait is computed by summing the `waitSeconds` across consecutive log lines with `status=WAITING` and the same `iter=<N>`.
- If no newer review and we are still under the cumulative cap, call `ScheduleWakeup` again for another `waitSeconds`.

**Do not block on `sleep`.** `ScheduleWakeup` is the correct mechanism and keeps the prompt cache warm between wakes (as long as `waitSeconds <= 270` or is a cold-cache-acceptable value like 600+; defaults respect this).

### Step 6 — Exit statuses and what to tell the user

One of these terminal statuses ends the loop. Each writes one log line and one message.

| Status | When | Message to user |
|---|---|---|
| `CONVERGED` | Step 2: Codex open threads = 0. | "PR #<N> is review-clean on Codex (0 open threads). Ready to merge. Loop exited CONVERGED after <N> iterations." |
| `MAX_ITERATIONS` | Step 0: reached `maxIterations`. | "Loop hit the max-iterations cap (<maxIterations>) without convergence. Latest open Codex thread count: <OPEN_CODEX>. Review the remaining threads manually, then re-run the loop if appropriate." |
| `NO_PROGRESS` | Step 3: fix skill made no changes (fixed=0, dismissed=0). | "Loop stopped on iteration <N>: the fix skill applied no changes this round. Either all open Codex findings are stale, or the only remedies would violate the constitution. Review the PR manually." |
| `GUARD_FAIL` | Step 4: trigger skill's guard pre-flight did not return READY TO PUSH. | "Loop stopped on iteration <N>: the constitution guard flagged P1s that block the next trigger. Fix the P1s (see the guard report), then re-run the loop." |
| `TIMEOUT` | Step 5: Codex did not post a new review within `maxWaitPerRoundSeconds`. | "Loop stopped on iteration <N>: Codex did not respond within <maxWaitPerRoundSeconds>s after the last trigger. The triggers are posted; run `/pr-review-loop-my-gather` again when Codex has responded, and the loop will pick up from the current state." |

Every terminal status appends one log line:

```
<ts> loop branch=<BRANCH> pr=<PR_NUMBER> iter=<N> status=<STATUS> open_codex=<OPEN_CODEX>
```

## Non-negotiable rules

- **Never reimplement fix or trigger logic here.** The two companion skills are the canonical implementations (Principle XIII). This skill is an orchestrator.
- **Never bypass `@agent-pre-review-constitution-guard`.** The guard pre-flight inside the trigger skill must return `READY TO PUSH` before a new round starts. If the user previously waived the guard for a single trigger, they MUST re-invoke the loop explicitly — the waiver does not carry across iterations.
- **Never retry a principle-dismissed finding with a looser remedy.** If XIII or XIV caused the fix skill to dismiss a finding in iteration N, that finding stays dismissed in iteration N+1 and beyond. The loop is not an escape hatch for the constitution.
- **Never override `maxIterations` silently.** If the user asks the loop to "keep going until Codex is happy", the skill still applies the cap — and the cap's purpose is explained in the `MAX_ITERATIONS` message so the user can re-invoke with a higher cap if they genuinely want more rounds.
- **Never bulk-delete review comments.** Thread-resolution is the fix skill's job; this loop inherits that behaviour. No wholesale cleanup.
- **Never post more than one pair of triggers per iteration.** That is the trigger skill's invariant; honour it.
- **Every artifact produced by this loop is English** (Principle XIV) — commit messages, log lines, review replies, status reports.

## What you tell the user on each wake-up

Keep it one short paragraph per wake-up so the conversation stays readable:

> "Iteration <N+1>/<maxIterations>: fix skill applied <M> fixes, dismissed <K>, resolved <R> threads. Trigger posted. Sleeping <waitSeconds>s for Codex to respond."

On the terminal wake-up, use the status-specific message from Step 6.

## Interaction with the `/loop` skill

This skill uses `ScheduleWakeup` internally, so it does **not** need to be wrapped in `/loop`. The difference:

- `/pr-review-loop-my-gather` (direct) — self-schedules until convergence or a safety rail. **Default and recommended.**
- `/loop Nm /pr-review-loop-my-gather` — the `/loop` skill fires the full convergence loop every N minutes regardless of whether the last one finished. This usually produces duplicate work or racing triggers; avoid unless the user has a specific reason.

## Convergence metric

Alongside the per-iteration log lines, append one final summary line on terminal statuses:

```
<ts> loop_summary branch=<BRANCH> pr=<PR_NUMBER> total_iterations=<N> final_status=<STATUS> total_fixed=<M_sum> total_dismissed=<K_sum> total_resolved=<R_sum>
```

Use this to measure whether the loop is hitting the max-iterations cap often. If the average ratio of `final_status=CONVERGED` stays above ~80% of runs with default caps, the workflow is healthy. If it drops, raise the cap or tighten the fix skill's anti-patterns list — do not silently loosen the principle-walk.
