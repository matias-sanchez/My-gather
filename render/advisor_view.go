package render

import (
	"fmt"
	"strings"

	"github.com/matias-sanchez/My-gather/findings"
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
				Value: formatNum(m.Value),
				Unit:  m.Unit,
				Note:  m.Note,
			})
		}
		out = append(out, findingView{
			ID:              f.ID,
			Subsystem:       f.Subsystem,
			Title:           f.Title,
			SeverityClass:   class,
			SeverityLabel:   label,
			OpenByDefault:   f.Severity == findings.SeverityCrit,
			Summary:         f.Summary,
			Explanation:     f.Explanation,
			FormulaText:     f.FormulaText,
			FormulaComputed: f.FormulaComputed,
			Metrics:         metrics,
			Recommendations: f.Recommendations,
		})
	}
	return out
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
