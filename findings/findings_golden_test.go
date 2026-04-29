package findings_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"

	"github.com/matias-sanchez/My-gather/findings"
	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestGoldenAdvisor is the Advisor-output golden: it runs the full
// pipeline (parse.Discover -> findings.Analyze) against
// testdata/example2/ and snapshot-compares the rendered Findings
// against testdata/golden/findings.example2.json.
//
// The golden is a compact, sorted-by-ID JSON projection of the
// Findings slice — only the metadata fields that matter for review
// (ID, Subsystem, category, confidence, FormulaText, evidence count,
// recommendation count, and relation count) so the file stays small
// enough to read in code review and is
// resilient to incidental wording changes inside Summary /
// Explanation.
func TestGoldenAdvisor(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixtureDir := filepath.Join(root, "testdata", "example2")

	c, err := parse.Discover(context.Background(), fixtureDir, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("parse.Discover(%s): %v", fixtureDir, err)
	}
	if c == nil {
		t.Fatalf("parse.Discover returned nil Collection")
	}

	report := buildReportForFindings(c)
	got := findings.Analyze(report)

	type goldenRow struct {
		ID                 string `json:"id"`
		Subsystem          string `json:"subsystem"`
		Title              string `json:"title"`
		Category           string `json:"category"`
		Severity           string `json:"severity"`
		Confidence         string `json:"confidence"`
		CoverageTopic      string `json:"coverage_topic"`
		FormulaText        string `json:"formula_text"`
		EvidenceLen        int    `json:"evidence_len"`
		RecommendationsLen int    `json:"recommendations_len"`
		RelatedLen         int    `json:"related_len"`
	}
	rows := make([]goldenRow, 0, len(got))
	for _, f := range got {
		rows = append(rows, goldenRow{
			ID:                 f.ID,
			Subsystem:          f.Subsystem,
			Title:              f.Title,
			Category:           string(f.Category),
			Severity:           severityName(f.Severity),
			Confidence:         string(f.Confidence),
			CoverageTopic:      f.CoverageTopic,
			FormulaText:        f.FormulaText,
			EvidenceLen:        len(f.Evidence),
			RecommendationsLen: len(f.RecommendationItems),
			RelatedLen:         len(f.RelatedFindings),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	body, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body = append(body, '\n')

	goldenPath := filepath.Join(root, "testdata", "golden", "findings.example2.json")
	goldens.Compare(t, goldenPath, body)
}

// severityName maps findings.Severity to a stable lowercase string
// for the golden JSON. The Severity int values are private-ish (an
// enum with iota), so the textual mapping insulates the golden from
// any future renumbering.
func severityName(s findings.Severity) string {
	switch s {
	case findings.SeverityCrit:
		return "crit"
	case findings.SeverityWarn:
		return "warn"
	case findings.SeverityInfo:
		return "info"
	case findings.SeverityOK:
		return "ok"
	default:
		return "unknown"
	}
}

// buildReportForFindings constructs the slim *model.Report that
// findings.Analyze actually reads (DBSection.Mysqladmin,
// DBSection.InnoDBPerSnapshot, VariablesSection.PerSnapshot). It
// mirrors the relevant pieces of render.buildReport without taking a
// dependency on the unexported render internals: render's full
// builder also computes Navigation, Environment, OSSection, and a
// ReportID, none of which findings.Analyze touches.
func buildReportForFindings(c *model.Collection) *model.Report {
	rpt := &model.Report{Collection: c}

	// VariablesSection: one entry per snapshot in capture order.
	var vs model.VariablesSection
	for _, s := range c.Snapshots {
		entry := model.SnapshotVariables{
			SnapshotPrefix: s.Prefix,
			Timestamp:      s.Timestamp,
		}
		if sf := s.SourceFiles[model.SuffixVariables]; sf != nil {
			if data, ok := sf.Parsed.(*model.VariablesData); ok {
				entry.Data = data
			}
		}
		vs.PerSnapshot = append(vs.PerSnapshot, entry)
	}
	rpt.VariablesSection = &vs

	// DBSection: per-snapshot InnoDB scalars + concatenated mysqladmin.
	dbs := &model.DBSection{}
	for _, s := range c.Snapshots {
		entry := model.SnapshotInnoDB{
			SnapshotPrefix: s.Prefix,
			Timestamp:      s.Timestamp,
		}
		if sf := s.SourceFiles[model.SuffixInnodbStatus]; sf != nil {
			if data, ok := sf.Parsed.(*model.InnodbStatusData); ok {
				entry.Data = data
			}
		}
		dbs.InnoDBPerSnapshot = append(dbs.InnoDBPerSnapshot, entry)
	}
	dbs.Mysqladmin = model.MergeMysqladminData(mysqladminInputs(c))
	rpt.DBSection = dbs
	return rpt
}

func mysqladminInputs(c *model.Collection) []*model.MysqladminData {
	var inputs []*model.MysqladminData
	for _, s := range c.Snapshots {
		sf := s.SourceFiles[model.SuffixMysqladmin]
		if sf == nil || sf.Parsed == nil {
			continue
		}
		data, ok := sf.Parsed.(*model.MysqladminData)
		if !ok || data == nil || data.SampleCount == 0 {
			continue
		}
		inputs = append(inputs, data)
	}
	return inputs
}
