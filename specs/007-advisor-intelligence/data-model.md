# Data Model: Advisor Intelligence

## Advisor Signal

Represents one raw or derived observation that can support Advisor output.

**Fields**

- `Subsystem`: canonical MySQL subsystem label.
- `Category`: diagnostic category: `Utilization`, `Saturation`, `Error`, or
  `Combined`.
- `Name`: metric name, processlist fact, wait-state fact, or error signal.
- `Value`: observed value in display-ready numeric or textual form.
- `Unit`: optional unit such as `%`, `/s`, `bytes`, `count`, or `state`.
- `Window`: capture window or timestamp range used for the signal.
- `EvidenceKind`: direct measurement, derived rate, derived ratio, observed
  state, observed error, or inference.
- `Strength`: evidence strength: `Strong`, `Moderate`, or `Weak`.

**Validation rules**

- A visible non-OK finding must include at least one strong signal or at least
  two moderate signals.
- Signals derived from absent or malformed inputs are not created.
- Derived signals must identify the direct inputs used to compute them.

## Diagnostic Finding

Represents one user-visible Advisor conclusion.

**Fields**

- `ID`: stable identifier used for deterministic ordering and references.
- `Subsystem`: canonical subsystem label.
- `Category`: diagnostic category from the dominant signal.
- `Severity`: `Critical`, `Warning`, `Info`, or `OK`.
- `Confidence`: `High`, `Medium`, or `Low`.
- `Title`: concise user-facing finding name.
- `Summary`: closed-card summary.
- `Interpretation`: plain-language explanation of why the evidence matters.
- `Evidence`: ordered Evidence Bundle.
- `Recommendations`: ordered confirmation and investigation actions.
- `RelatedFindingIDs`: stable IDs for correlated findings.
- `CoverageTopic`: Rosetta Stone topic represented by the finding.

**Validation rules**

- `ID` values must be unique and stable.
- Visible findings must have non-empty subsystem, category, title, summary,
  interpretation, and source coverage topic.
- Non-OK findings must include evidence and recommendations.
- Missing required evidence must produce a skipped or downgraded finding rather
  than a warning without support.
- Finding order must be deterministic for the same report.

## Evidence Bundle

Represents the facts displayed to justify a finding.

**Fields**

- `Signals`: ordered Advisor Signals.
- `ComputedText`: optional human-readable formula with substituted values.
- `DirectEvidenceCount`: number of direct measurements or observed facts.
- `InferenceCount`: number of inferred relationships.
- `MissingInputs`: optional list of expected inputs that were unavailable.

**Validation rules**

- Direct evidence must appear before inferred interpretation.
- Missing inputs may appear only as context and must not be used to escalate
  severity.
- Evidence ordering must be stable and meaningful to a human reader.

## Recommendation

Represents one next step shown under a finding.

**Fields**

- `Kind`: `Confirm`, `Investigate`, `Mitigate`, or `Caution`.
- `Text`: concise action for the analyst.
- `AppliesWhen`: optional condition that scopes the action.
- `RelatedEvidence`: signal names that justify the recommendation.

**Validation rules**

- Every non-OK finding must include at least one confirmation step.
- Mitigation recommendations must be scoped to evidence shown in the finding.
- Recommendations must not imply unavailable data was observed.

## Finding Correlation

Represents a relationship between findings that helps prioritize incident
drivers.

**Fields**

- `PrimaryFindingID`: stable ID of the dominant finding.
- `RelatedFindingID`: stable ID of the supporting or secondary finding.
- `Relationship`: `Supports`, `MayExplain`, `MayBeCausedBy`, or `SamePressure`.
- `Reason`: concise user-facing explanation.

**Validation rules**

- Correlations must use stable finding IDs that exist in the visible finding
  set.
- Correlations must not hide critical secondary findings.
- Correlations must be deterministic for the same finding set.

## Expert Coverage Map

Documents how the feature covers the Rosetta Stone source topics.

**Fields**

- `Topic`: expert-source topic name.
- `Subsystem`: canonical Advisor subsystem.
- `Category`: utilization, saturation, error, or combined.
- `CoverageStatus`: `Covered`, `Partial`, `Deferred`, or `Excluded`.
- `Signals`: metrics or observed facts used when covered.
- `Reason`: explanation for partial, deferred, or excluded topics.

**Validation rules**

- Every selected Rosetta Stone topic must have exactly one coverage row.
- Deferred topics must identify the missing report evidence or product scope
  reason.
- Excluded topics must explain why the topic is outside this feature's scope.
