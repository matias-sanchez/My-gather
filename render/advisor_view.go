package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/matias-sanchez/My-gather/findings"
	"github.com/matias-sanchez/My-gather/reportutil"
)

// --- Advisor / findings view helpers --------------------------------

func buildFindingViews(fs []findings.Finding) []findingView {
	out := make([]findingView, 0, len(fs))
	for _, f := range fs {
		class, label := severityUIFields(f.Severity)
		metrics := make([]findingMetricView, 0, len(f.Metrics))
		for _, m := range f.Metrics {
			metrics = append(metrics, findingMetricView{
				Name:  m.Name,
				Value: reportutil.FormatNum(m.Value),
				Unit:  m.Unit,
				Note:  m.Note,
			})
		}
		evidence := make([]findingEvidenceView, 0, len(f.Evidence))
		for _, e := range f.Evidence {
			evidence = append(evidence, findingEvidenceView{
				Name:     e.Name,
				Value:    e.Value,
				Unit:     e.Unit,
				Kind:     string(e.Kind),
				Strength: string(e.Strength),
				Note:     e.Note,
			})
		}
		recommendations := make([]findingRecommendationView, 0, len(f.RecommendationItems))
		for _, rec := range f.RecommendationItems {
			recommendations = append(recommendations, findingRecommendationView{
				Kind:            string(rec.Kind),
				Text:            rec.Text,
				AppliesWhen:     rec.AppliesWhen,
				RelatedEvidence: append([]string(nil), rec.RelatedEvidence...),
			})
		}
		related := make([]findingRelatedView, 0, len(f.RelatedFindings))
		for _, rel := range f.RelatedFindings {
			related = append(related, findingRelatedView{
				ID:           rel.ID,
				Relationship: string(rel.Relationship),
				Reason:       rel.Reason,
			})
		}
		out = append(out, findingView{
			ID:              f.ID,
			Subsystem:       f.Subsystem,
			Title:           f.Title,
			Category:        string(f.Category),
			Confidence:      string(f.Confidence),
			SeverityClass:   class,
			SeverityLabel:   label,
			OpenByDefault:   f.Severity == findings.SeverityCrit,
			Summary:         f.Summary,
			Explanation:     f.Explanation,
			FormulaText:     f.FormulaText,
			FormulaComputed: f.FormulaComputed,
			Metrics:         metrics,
			Evidence:        evidence,
			Recommendations: recommendations,
			RelatedFindings: related,
			CoverageTopic:   f.CoverageTopic,
		})
	}
	return out
}

func buildTopDriverViews(fs []findings.Finding) []findingDriverView {
	type rankedFinding struct {
		f     findings.Finding
		score int
	}
	ranked := make([]rankedFinding, 0, len(fs))
	for _, f := range fs {
		if f.Severity == findings.SeverityOK {
			continue
		}
		ranked = append(ranked, rankedFinding{f: f, score: driverScore(f)})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if findings.SubsystemOrder(ranked[i].f.Subsystem) != findings.SubsystemOrder(ranked[j].f.Subsystem) {
			return findings.SubsystemOrder(ranked[i].f.Subsystem) < findings.SubsystemOrder(ranked[j].f.Subsystem)
		}
		return ranked[i].f.ID < ranked[j].f.ID
	})
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}
	out := make([]findingDriverView, 0, len(ranked))
	for _, item := range ranked {
		class, label := severityUIFields(item.f.Severity)
		out = append(out, findingDriverView{
			ID:            item.f.ID,
			Subsystem:     item.f.Subsystem,
			Title:         item.f.Title,
			Category:      string(item.f.Category),
			Confidence:    string(item.f.Confidence),
			SeverityClass: class,
			SeverityLabel: label,
			Summary:       item.f.Summary,
			Why:           driverReason(item.f),
		})
	}
	return out
}

func driverScore(f findings.Finding) int {
	score := 0
	switch f.Severity {
	case findings.SeverityCrit:
		score += 300
	case findings.SeverityWarn:
		score += 200
	case findings.SeverityInfo:
		score += 100
	}
	switch f.Confidence {
	case findings.ConfidenceHigh:
		score += 30
	case findings.ConfidenceMedium:
		score += 20
	case findings.ConfidenceLow:
		score += 10
	}
	score += minInt(len(f.Evidence), 4) * 3
	score += minInt(len(f.RelatedFindings), 3) * 2
	return score
}

func driverReason(f findings.Finding) string {
	parts := []string{string(f.Category), string(f.Confidence) + " confidence"}
	if len(f.Evidence) > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", len(f.Evidence), plural(len(f.Evidence), "evidence row", "evidence rows")))
	}
	if len(f.RelatedFindings) > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", len(f.RelatedFindings), plural(len(f.RelatedFindings), "related finding", "related findings")))
	}
	return strings.Join(parts, " - ")
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func severityUIFields(s findings.Severity) (class, label string) {
	switch s {
	case findings.SeverityCrit:
		return "crit", "Critical"
	case findings.SeverityWarn:
		return "warn", "Warning"
	case findings.SeverityInfo:
		return "info", "Info"
	case findings.SeverityOK:
		return "ok", "OK"
	}
	return "info", "Info"
}

func summariseFindings(fs []findings.Finding) findingCountsView {
	c := findings.Summarise(fs)
	return findingCountsView{
		Crit: c.Crit,
		Warn: c.Warn,
		Info: c.Info,
		OK:   c.OK,
		Any:  (c.Crit + c.Warn + c.Info + c.OK) > 0,
	}
}

func advisorBadge(c findingCountsView) string {
	if !c.Any {
		return "no findings"
	}
	parts := []string{}
	if c.Crit > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", c.Crit))
	}
	if c.Warn > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", c.Warn))
	}
	if c.Info > 0 {
		parts = append(parts, fmt.Sprintf("%d info", c.Info))
	}
	if c.OK > 0 {
		parts = append(parts, fmt.Sprintf("%d ok", c.OK))
	}
	return strings.Join(parts, " · ")
}
