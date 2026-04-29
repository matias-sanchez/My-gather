# Phase 1 Package Contracts: Advisor Intelligence

## `findings.Analyze`

Input: a `*model.Report` containing parsed pt-stalk data.

Output: a deterministic slice of visible Advisor findings.

Additional contract:

- Continue to skip rules whose required inputs are unavailable.
- Preserve deterministic ordering for the same report.
- Return findings with subsystem, diagnostic category, severity, confidence,
  evidence, recommendations, and source coverage metadata.
- Keep the native rule registry as the only Advisor dispatch path.
- Avoid duplicate or contradictory findings by emitting deterministic
  correlation metadata or supporting references.
- Never perform filesystem or network lookups while evaluating report data.

## `findings.Registry`

Input: none.

Output: a deterministic copy of registered rule metadata.

Additional contract:

- Include diagnostic category and coverage topic metadata for each registered
  rule.
- Preserve unique stable IDs for every rule.
- Support quality tests that verify metadata completeness, recommendations,
  subsystem validity, and coverage traceability.

## Advisor rule functions

Input: a `*model.Report`.

Output: one finding or a skipped result.

Additional contract:

- Rules must distinguish utilization, saturation, and error signals.
- Rules must list all direct metrics or observed facts used as evidence.
- Rules must downgrade or skip when required evidence is absent.
- Rules must provide confirmation checks and investigation guidance for every
  non-OK visible finding.
- Rules must not infer unavailable evidence or escalate severity from missing
  inputs.

## `findings` Advisor types

Input: values produced by `findings`.

Output: typed structures used by render and tests.

Additional contract:

- Expose stable fields for diagnostic category, confidence, evidence bundle,
  recommendations, related finding IDs, and coverage topic.
- Keep exported identifiers documented.
- Keep fields serializable through existing deterministic report rendering.

## `render.buildFindingViews`

Input: Advisor findings from `findings.Analyze`.

Output: template-facing finding views.

Additional contract:

- Preserve visible severity labels and existing severity filters.
- Render diagnostic category and confidence without replacing severity.
- Render direct evidence before inferred interpretation.
- Render recommendations in deterministic order.
- Render related finding references when present.
- Keep critical findings open by default unless an existing report rule says
  otherwise.

## Advisor golden output

Input: curated report fixtures and focused synthetic report models.

Output: committed deterministic golden artifacts.

Additional contract:

- Goldens must capture rule ID, subsystem, category, severity, confidence,
  source coverage topic, evidence count, and recommendation count.
- HTML goldens must capture the rendered Advisor summary and card anatomy.
- Golden updates must be explicit and reviewed.
