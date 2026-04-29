# Quickstart: Advisor Intelligence

## 1. Review the active feature

```bash
cat .specify/feature.json
sed -n '1,220p' specs/007-advisor-intelligence/spec.md
sed -n '1,220p' specs/007-advisor-intelligence/plan.md
```

Expected result: `.specify/feature.json` points to
`specs/007-advisor-intelligence`, and the spec plus plan describe Advisor
intelligence, not a parser or new collector feature.

## 2. Inspect the current Advisor baseline

```bash
go test ./findings -count=1
go test ./render -run Advisor -count=1
```

Expected result: existing Advisor rules and render tests pass before changes.

## 3. Validate rule metadata quality during implementation

```bash
go test ./findings -run 'TestRuleQuality|TestGoldenAdvisor' -count=1
```

Expected result: every registered rule has stable metadata, known subsystem,
recommendations, and deterministic golden output.

## 4. Validate full report rendering

```bash
go test ./render -count=1
```

Expected result: the Advisor section renders summary counts, top drivers,
finding cards, evidence bundles, and related finding references without
breaking existing report output.

## 5. Run full local gates before review

```bash
go vet ./...
go test ./... -count=1
make lint
scripts/hooks/pre-push-constitution-guard.sh
```

Expected result: all commands pass before the branch is pushed or reviewed.

## 6. Manual report check

Generate a report from a known incident fixture and inspect the Advisor:

```bash
go run ./cmd/my-gather --overwrite -o /tmp/advisor-intelligence.html testdata/example2
```

Review checklist:

- Critical and warning findings are easy to identify.
- Top suspected drivers are visible near the start of the Advisor section.
- Each non-OK finding shows evidence, interpretation, and next checks.
- Missing inputs are not presented as observed evidence.
- Related findings are cross-referenced without hiding important cards.
- The report works offline as a single HTML file.
