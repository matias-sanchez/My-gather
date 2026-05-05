package render

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// ErrNilCollection is returned by Render when the supplied Collection
// is nil. Exposed as a sentinel so callers can distinguish the bad-
// input case from other rendering errors (e.g., via errors.Is).
var ErrNilCollection = errors.New("render: nil Collection")

// RenderOptions controls optional aspects of rendering. The zero value
// is valid and derives GeneratedAt from the input Collection so report
// output remains deterministic.
type RenderOptions struct {
	// GeneratedAt is the report timestamp. When omitted, Render derives
	// it from the first non-zero Snapshot timestamp in the input.
	GeneratedAt time.Time

	// Version is the tool's semver string ("v0.1.0").
	Version string

	// GitCommit is the short git SHA.
	GitCommit string

	// BuiltAt is the build timestamp (injected via -ldflags). Purely
	// informational; never rendered in a way that affects layout.
	BuiltAt string
}

// Render writes a self-contained HTML report for c to w. All CSS,
// JavaScript, fonts, and data are embedded inline; the resulting file
// makes zero network requests at view time (Constitution Principle V,
// spec FR-004).
//
// Render is deterministic: given the same Collection and RenderOptions,
// two invocations MUST write byte-identical output to w (Constitution
// Principle IV, spec FR-006).
//
// Render returns an error only for I/O failures against w or for
// fatal template parsing errors. A Collection with missing or failed
// per-collector data is never an error — the corresponding section
// renders its "data not available" banner.
func Render(w io.Writer, c *model.Collection, opts RenderOptions) error {
	if c == nil {
		return ErrNilCollection
	}
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = deterministicGeneratedAt(c)
	}

	report, sigs := buildReport(c, opts)
	view, err := buildView(report, c, sigs)
	if err != nil {
		return fmt.Errorf("render: build view: %w", err)
	}

	tmpl, err := template.ParseFS(embeddedTemplates, "templates/*.tmpl")
	if err != nil {
		return fmt.Errorf("render: parse templates: %w", err)
	}

	// Render into a buffer first so a failure doesn't leave the caller's
	// writer partially written.
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "report.html.tmpl", view); err != nil {
		return fmt.Errorf("render: execute template: %w", err)
	}
	if _, err := io.Copy(w, &buf); err != nil {
		return fmt.Errorf("render: write output: %w", err)
	}
	return nil
}

func deterministicGeneratedAt(c *model.Collection) time.Time {
	for _, snap := range c.Snapshots {
		if snap != nil && !snap.Timestamp.IsZero() {
			return snap.Timestamp.UTC()
		}
	}
	return time.Time{}
}

// --- Internal view construction ---------------------------------------------

// buildReport converts a Collection into the typed Report envelope.
// It also returns the per-snapshot variable-signature cache so buildView
// can reuse it without re-hashing (NIT #27).
func buildReport(c *model.Collection, opts RenderOptions) (*model.Report, []string) {
	rpt := &model.Report{
		Title:       collectionTitle(c),
		Version:     opts.Version,
		GitCommit:   opts.GitCommit,
		BuiltAt:     opts.BuiltAt,
		GeneratedAt: opts.GeneratedAt.UTC(),
		Collection:  c,
		ReportID:    canonicalReportID(c),
	}
	rpt.OSSection = buildOSSection(c)
	rpt.VariablesSection = buildVariablesSection(c)
	rpt.DBSection = buildDBSection(c)
	rpt.EnvironmentSection = buildEnvironmentSection(c)
	sigs := computeVariableSignatures(rpt.VariablesSection)
	rpt.Navigation = buildNavigation(rpt, sigs)
	return rpt, sigs
}
