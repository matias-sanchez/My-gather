# Phase 1 UI Contract: Advisor Intelligence

## Advisor Summary

The Advisor section must begin with a compact summary that helps an analyst
identify the most important incident drivers quickly.

Required content:

- Counts for Critical, Warning, Info, and OK findings.
- A deterministic top-driver list when non-OK findings exist.
- Each top driver shows subsystem, severity, diagnostic category, title, and a
  short reason.
- Sparse captures show a clear message when rules had insufficient evidence.

Behavior:

- Summary content is rendered in the self-contained HTML report.
- Summary does not fetch remote assets or data.
- Summary order is deterministic for the same report.
- Top-driver ranking is derived from severity, confidence, evidence count, and
  relation count, with stable rule-ID tie breaking.

## Finding Cards

Each visible finding card must show:

- Severity label.
- Diagnostic category label.
- Confidence label.
- Subsystem.
- Title and summary.
- Plain-language interpretation.
- Evidence bundle with direct evidence before inferred interpretation.
- Recommended confirmation and investigation steps.
- Related findings when present.
- Source coverage topic.

Behavior:

- Critical findings open by default.
- Warning, Info, and OK findings may remain collapsed by default.
- Existing severity filters continue to work.
- The card remains readable when evidence or recommendations are longer than
  one line.
- Category and confidence appear as compact chips beside the severity label.

## Evidence Display

Evidence rows must be compact and scannable.

Required content:

- Evidence name.
- Value and unit when available.
- Evidence kind.
- Optional note for derived values or missing context.

Behavior:

- Missing evidence is never rendered as if it were observed.
- Derived values identify the direct inputs used in the finding text or formula.
- Ordering is stable and meaningful.
- Derived rates and ratios are labelled separately from direct measurements.

## Correlated Findings

When findings are related, the UI must make the relationship clear without
hiding supporting findings.

Required content:

- Related finding title or stable identifier.
- Relationship label such as supports, may explain, may be caused by, or same
  pressure.
- Concise reason for the relationship.

Behavior:

- Related links or references stay inside the generated report.
- Critical related findings remain visible as normal findings.
- Relationships are additive references. They never suppress or merge the
  target finding card.

## Empty and Healthy States

The Advisor must remain useful when few or no rules fire.

Required content:

- If no findings are visible, show that every analytical rule either passed or
  lacked enough input.
- If only OK or informational findings are visible, preserve the ability to
  inspect them without making the report look urgent.

Behavior:

- Empty and healthy states are deterministic.
- Sparse-input explanations do not imply missing collectors are failures.
