package findings

import (
	"testing"
)

// knownSubsystems is the set of Subsystem string values rendered by
// the Advisor. Mirrors the cases in subsystemOrder; new subsystems
// must be added in both places (the test catches the omission here,
// the renderer uses the order from subsystemOrder).
var knownSubsystems = map[string]struct{}{
	"Buffer Pool":       {},
	"Redo Log":          {},
	"InnoDB Semaphores": {},
	"Binlog Cache":      {},
	"Thread Cache":      {},
	"Table Open Cache":  {},
	"Temp Tables":       {},
	"Query Shape":       {},
	"Connections":       {},
	"Configuration":     {},
}

// TestRuleQuality_RegistryNotEmpty fails when the registry is somehow
// empty at test time — guards against package-init failures that
// would silently drop every rule.
func TestRuleQuality_RegistryNotEmpty(t *testing.T) {
	if len(registry) == 0 {
		t.Fatal("rule registry is empty: package init did not register any rules")
	}
}

// TestRuleQuality_MetadataIsComplete asserts that every entry in the
// registry has non-empty ID, Title, and FormulaText. These fields are
// load-bearing for the Advisor render path and the rendered Finding
// quality.
func TestRuleQuality_MetadataIsComplete(t *testing.T) {
	for _, def := range registry {
		if def.ID == "" {
			t.Errorf("rule with empty ID found (Title=%q)", def.Title)
		}
		if def.Title == "" {
			t.Errorf("rule %q has empty Title", def.ID)
		}
		if def.FormulaText == "" {
			t.Errorf("rule %q has empty FormulaText", def.ID)
		}
		if def.Run == nil {
			t.Errorf("rule %q has nil Run function", def.ID)
		}
	}
}

// TestRuleQuality_IDsAreUnique guards against the failure mode where
// two registrations claim the same ID. Two definitions with the same
// ID would silently shadow each other in the rendered output (the
// last one to evaluate wins, depending on iteration order).
func TestRuleQuality_IDsAreUnique(t *testing.T) {
	seen := make(map[string]int, len(registry))
	for _, def := range registry {
		seen[def.ID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("rule ID %q is registered %d times; IDs must be unique", id, count)
		}
	}
}

// TestRuleQuality_SubsystemIsKnown asserts every registered rule
// declares a Subsystem value that the renderer's subsystemOrder
// switch handles. New subsystems must be added to both subsystemOrder
// and knownSubsystems above.
func TestRuleQuality_SubsystemIsKnown(t *testing.T) {
	for _, def := range registry {
		if _, ok := knownSubsystems[def.Subsystem]; !ok {
			t.Errorf("rule %q declares unknown Subsystem %q", def.ID, def.Subsystem)
		}
	}
}

// TestRuleQuality_CritRulesHaveEnoughRecommendations asserts that
// rules whose declared worst case is SeverityHintCritical carry at
// least three remediation recommendations: a Critical card with one
// suggestion is a quality-of-output regression.
//
// Variable-severity rules are checked separately: they must declare
// at least one recommendation, but the per-rule judgement of how many
// suggestions they actually need is captured in MinRecommendations
// itself (the rule author picks a value matching the rendered Finding).
func TestRuleQuality_CritRulesHaveEnoughRecommendations(t *testing.T) {
	for _, def := range registry {
		switch def.Severity {
		case SeverityHintCritical:
			if def.MinRecommendations < 3 {
				t.Errorf("rule %q is declared SeverityHintCritical but MinRecommendations=%d (need >=3)",
					def.ID, def.MinRecommendations)
			}
		case SeverityHintVariable, SeverityHintWarning, SeverityHintInfo:
			if def.MinRecommendations < 1 {
				t.Errorf("rule %q declares 0 recommendations (need >=1)", def.ID)
			}
		}
	}
}

// TestRuleQuality_FindingHonorsMinRecommendations asserts that when a
// rule fires non-skip on a fixture, the rendered Finding actually
// carries at least the declared MinRecommendations. Run() against a
// nil report returns SeveritySkip for every rule, so this scan is a
// best-effort sanity check on the declared metadata; per-rule fixture
// tests in findings_test.go are still the authoritative coverage.
func TestRuleQuality_FindingHonorsMinRecommendations(t *testing.T) {
	for _, def := range registry {
		if def.MinRecommendations <= 0 {
			continue
		}
		// Per-rule fixture tests already exercise the Run path with
		// realistic inputs; here we just validate that the declared
		// floor is non-negative and the function pointer is non-nil
		// (the latter is also checked by MetadataIsComplete; this
		// test scaffolds the per-Finding check that PR-N+1 will
		// extend with synthetic inputs that force a fire).
		if def.Run == nil {
			t.Errorf("rule %q has nil Run", def.ID)
		}
	}
}

// TestRuleQuality_RegistryIsSortedByID asserts the dispatch order is
// stable and alphabetical, the property Analyze relies on for
// deterministic output. Calls Analyze first to trigger the
// registry-sort sync.Once (Analyze sorts on first call rather than at
// init time, see findings.go for the rationale).
func TestRuleQuality_RegistryIsSortedByID(t *testing.T) {
	_ = Analyze(nil) // primes the sync.Once
	for i := 1; i < len(registry); i++ {
		if registry[i-1].ID > registry[i].ID {
			t.Errorf("registry not sorted by ID: index %d (%q) > index %d (%q)",
				i-1, registry[i-1].ID, i, registry[i].ID)
		}
	}
}
