package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/chart.min.js
var embeddedChartJS string

//go:embed assets/chart.min.css
var embeddedChartCSS string

//go:embed assets/app.js
var embeddedAppJS string

//go:embed assets/app.css
var embeddedAppCSS string

// RenderOptions controls optional aspects of rendering. The zero value
// is valid and uses the current UTC time and empty Version/GitCommit.
type RenderOptions struct {
	// GeneratedAt is the single explicitly non-deterministic field in
	// the rendered HTML (Constitution Principle IV, spec FR-006).
	// Tests pass a fixed value to make golden comparisons stable.
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
// Render is deterministic: given the same Collection and RenderOptions
// (including GeneratedAt), two invocations MUST write byte-identical
// output to w (Constitution Principle IV, spec FR-006).
//
// Render returns an error only for I/O failures against w or for
// fatal template parsing errors. A Collection with missing or failed
// per-collector data is never an error — the corresponding section
// renders its "data not available" banner.
func Render(w io.Writer, c *model.Collection, opts RenderOptions) error {
	if c == nil {
		return fmt.Errorf("render: nil Collection")
	}
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = time.Now().UTC()
	}

	report := buildReport(c, opts)
	view := buildView(report, c)

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

// --- Internal view construction ---------------------------------------------

// buildReport converts a Collection into the typed Report envelope.
func buildReport(c *model.Collection, opts RenderOptions) *model.Report {
	rpt := &model.Report{
		Title:       collectionTitle(c),
		Version:     opts.Version,
		GitCommit:   opts.GitCommit,
		BuiltAt:     opts.BuiltAt,
		GeneratedAt: opts.GeneratedAt.UTC(),
		Collection:  c,
		ReportID:    CanonicalReportID(c),
	}
	rpt.OSSection = buildOSSection(c)
	rpt.VariablesSection = buildVariablesSection(c)
	rpt.DBSection = buildDBSection(c)
	rpt.Navigation = buildNavigation(rpt)
	return rpt
}

func collectionTitle(c *model.Collection) string {
	if c.Hostname != "" {
		return c.Hostname
	}
	if len(c.Snapshots) > 0 {
		return c.Snapshots[0].Prefix
	}
	return "unknown-collection"
}

// buildOSSection pulls OS-related parsed payloads out of the Collection.
// When a Snapshot is missing a collector, or Parsed is nil, we consider
// the subview "missing" and render the banner.
func buildOSSection(c *model.Collection) *model.OSSection {
	sec := &model.OSSection{}
	var io *model.IostatData
	var tp *model.TopData
	var vm *model.VmstatData
	for _, snap := range c.Snapshots {
		if sf, ok := snap.SourceFiles[model.SuffixIostat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.IostatData); ok && io == nil {
				io = v
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixTop]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.TopData); ok && tp == nil {
				tp = v
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixVmstat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.VmstatData); ok && vm == nil {
				vm = v
			}
		}
	}
	sec.Iostat = io
	sec.Top = tp
	sec.Vmstat = vm
	if io == nil {
		sec.Missing = append(sec.Missing, "-iostat")
	}
	if tp == nil {
		sec.Missing = append(sec.Missing, "-top")
	}
	if vm == nil {
		sec.Missing = append(sec.Missing, "-vmstat")
	}
	sort.Strings(sec.Missing)
	return sec
}

func buildVariablesSection(c *model.Collection) *model.VariablesSection {
	sec := &model.VariablesSection{}
	for _, snap := range c.Snapshots {
		sv := model.SnapshotVariables{
			SnapshotPrefix: snap.Prefix,
			Timestamp:      snap.Timestamp,
		}
		if sf, ok := snap.SourceFiles[model.SuffixVariables]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.VariablesData); ok {
				sv.Data = v
			}
		}
		sec.PerSnapshot = append(sec.PerSnapshot, sv)
	}
	return sec
}

func buildDBSection(c *model.Collection) *model.DBSection {
	sec := &model.DBSection{}
	for _, snap := range c.Snapshots {
		si := model.SnapshotInnoDB{
			SnapshotPrefix: snap.Prefix,
			Timestamp:      snap.Timestamp,
		}
		if sf, ok := snap.SourceFiles[model.SuffixInnodbStatus]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.InnodbStatusData); ok {
				si.Data = v
			}
		}
		sec.InnoDBPerSnapshot = append(sec.InnoDBPerSnapshot, si)
	}

	// Mysqladmin and Processlist: use the first non-nil parsed payload
	// encountered. US4 will extend this to merge-across-snapshots; for
	// the MVP render skeleton the single-snapshot case is enough.
	for _, snap := range c.Snapshots {
		if sec.Mysqladmin == nil {
			if sf, ok := snap.SourceFiles[model.SuffixMysqladmin]; ok && sf.Parsed != nil {
				if v, ok := sf.Parsed.(*model.MysqladminData); ok {
					sec.Mysqladmin = v
				}
			}
		}
		if sec.Processlist == nil {
			if sf, ok := snap.SourceFiles[model.SuffixProcesslist]; ok && sf.Parsed != nil {
				if v, ok := sf.Parsed.(*model.ProcesslistData); ok {
					sec.Processlist = v
				}
			}
		}
	}

	if allInnoDBNil(sec.InnoDBPerSnapshot) {
		sec.Missing = append(sec.Missing, "-innodbstatus1")
	}
	if sec.Mysqladmin == nil {
		sec.Missing = append(sec.Missing, "-mysqladmin")
	}
	if sec.Processlist == nil {
		sec.Missing = append(sec.Missing, "-processlist")
	}
	sort.Strings(sec.Missing)
	return sec
}

func allInnoDBNil(xs []model.SnapshotInnoDB) bool {
	for _, x := range xs {
		if x.Data != nil {
			return false
		}
	}
	return true
}

// buildNavigation produces a flat, deterministic list of navigation
// entries. Level-1 and Level-2 entries interleave in the order the
// templates render them.
func buildNavigation(r *model.Report) []model.NavEntry {
	var nav []model.NavEntry

	// OS section.
	nav = append(nav, model.NavEntry{ID: "sec-os", Title: "OS Usage", Level: 1})
	nav = append(nav, model.NavEntry{ID: "sub-os-iostat", Title: "Disk utilization", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-top", Title: "Top CPU processes", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-vmstat", Title: "vmstat saturation", Level: 2, ParentID: "sec-os"})

	// Variables section (one Level-2 per snapshot).
	nav = append(nav, model.NavEntry{ID: "sec-variables", Title: "Variables", Level: 1})
	if r.VariablesSection != nil {
		for i, sv := range r.VariablesSection.PerSnapshot {
			nav = append(nav, model.NavEntry{
				ID:       variablesSnapshotID(i),
				Title:    sv.SnapshotPrefix,
				Level:    2,
				ParentID: "sec-variables",
			})
		}
	}

	// DB section.
	nav = append(nav, model.NavEntry{ID: "sec-db", Title: "Database Usage", Level: 1})
	nav = append(nav, model.NavEntry{ID: "sub-db-innodb", Title: "InnoDB status", Level: 2, ParentID: "sec-db"})
	nav = append(nav, model.NavEntry{ID: "sub-db-mysqladmin", Title: "Counter deltas", Level: 2, ParentID: "sec-db"})
	nav = append(nav, model.NavEntry{ID: "sub-db-processlist", Title: "Thread states", Level: 2, ParentID: "sec-db"})

	// Diagnostics.
	nav = append(nav, model.NavEntry{ID: "sec-diagnostics", Title: "Parser Diagnostics", Level: 1})

	return nav
}

func variablesSnapshotID(idx int) string {
	return fmt.Sprintf("sub-var-%d", idx+1)
}

// --- Template view (what the .tmpl files actually see) ----------------------

// reportView is the value passed to the top-level template. It
// flattens the Report into fields the template can consume without
// complex logic.
type reportView struct {
	Title              string
	Hostname           string
	Version            string
	GitCommit          string
	GeneratedAtDisplay string
	SnapshotCount      int
	Navigation         []model.NavEntry

	EmbeddedCSS     template.CSS
	EmbeddedChartJS template.JS
	EmbeddedAppJS   template.JS
	DataPayload     template.JS // JSON, emitted inside <script type="application/json">

	OSBadge          string
	VariablesBadge   string
	DBBadge          string
	DiagnosticsBadge string

	// OS section payload
	HasIostat bool
	HasTop    bool
	HasVmstat bool

	// Variables section payload
	HasVariables      bool
	VariableSnapshots []variableSnapshotView

	// DB section payload
	HasInnoDB           bool
	InnoDBSnapshots     []innoDBSnapshotView
	HasMysqladmin       bool
	MysqladminVariables []string
	MysqladminSelectID  string
	HasProcesslist      bool

	// Diagnostics (flattened for template iteration).
	Diagnostics []diagnosticView
}

type variableSnapshotView struct {
	DetailsID string
	Title     string
	Badge     string
	Count     int
	Data      *model.VariablesData
}

type innoDBSnapshotView struct {
	Title         string
	Data          *model.InnodbStatusData
	PendingTotal  int
	AHISearchRate string
}

type diagnosticView struct {
	SourceFileDisplay string
	Location          string
	SeverityClass     string
	SeverityLabel     string
	Message           string
}

// buildView flattens the Report into the template-friendly shape.
func buildView(r *model.Report, c *model.Collection) *reportView {
	v := &reportView{
		Title:              r.Title,
		Hostname:           c.Hostname,
		Version:            r.Version,
		GitCommit:          r.GitCommit,
		GeneratedAtDisplay: FormatTimestamp(r.GeneratedAt),
		SnapshotCount:      len(c.Snapshots),
		Navigation:         r.Navigation,
		EmbeddedCSS:        template.CSS(embeddedChartCSS + "\n" + embeddedAppCSS),
		EmbeddedChartJS:    template.JS(embeddedChartJS),
		EmbeddedAppJS:      template.JS(embeddedAppJS),
		MysqladminSelectID: "mysqladmin-select",
	}

	if r.OSSection != nil {
		v.HasIostat = r.OSSection.Iostat != nil
		v.HasTop = r.OSSection.Top != nil
		v.HasVmstat = r.OSSection.Vmstat != nil
	}
	presentOS := boolToInt(v.HasIostat) + boolToInt(v.HasTop) + boolToInt(v.HasVmstat)
	v.OSBadge = fmt.Sprintf("%d / 3 subviews", presentOS)

	if r.VariablesSection != nil {
		haveAny := false
		for i, sv := range r.VariablesSection.PerSnapshot {
			vv := variableSnapshotView{
				DetailsID: variablesSnapshotID(i),
				Title:     fmt.Sprintf("Snapshot %s", sv.SnapshotPrefix),
				Badge:     fmt.Sprintf("snap #%d", i+1),
				Data:      sv.Data,
			}
			if sv.Data != nil {
				vv.Count = len(sv.Data.Entries)
				haveAny = true
			}
			v.VariableSnapshots = append(v.VariableSnapshots, vv)
		}
		v.HasVariables = haveAny
	}
	if v.HasVariables {
		v.VariablesBadge = fmt.Sprintf("%d snapshots", len(v.VariableSnapshots))
	} else {
		v.VariablesBadge = "missing"
	}

	if r.DBSection != nil {
		for _, si := range r.DBSection.InnoDBPerSnapshot {
			iv := innoDBSnapshotView{
				Title: fmt.Sprintf("Snapshot %s", si.SnapshotPrefix),
				Data:  si.Data,
			}
			if si.Data != nil {
				iv.PendingTotal = si.Data.PendingReads + si.Data.PendingWrites
				iv.AHISearchRate = FormatFloat(si.Data.AHIActivity.SearchesPerSec, 2)
				v.HasInnoDB = true
			}
			v.InnoDBSnapshots = append(v.InnoDBSnapshots, iv)
		}
		v.HasMysqladmin = r.DBSection.Mysqladmin != nil
		v.HasProcesslist = r.DBSection.Processlist != nil
		if v.HasMysqladmin {
			v.MysqladminVariables = append(v.MysqladminVariables, r.DBSection.Mysqladmin.VariableNames...)
		}
	}
	presentDB := boolToInt(v.HasInnoDB) + boolToInt(v.HasMysqladmin) + boolToInt(v.HasProcesslist)
	v.DBBadge = fmt.Sprintf("%d / 3 subviews", presentDB)

	// Diagnostics: collection-wide + per-SourceFile, sorted stably by
	// (severity desc, source-file, location).
	v.Diagnostics = flattenDiagnostics(c)
	if len(v.Diagnostics) == 0 {
		v.DiagnosticsBadge = "clean"
	} else {
		v.DiagnosticsBadge = fmt.Sprintf("%d entries", len(v.Diagnostics))
	}

	// Build the embedded data payload. Per-chart series are populated
	// by US2-US4 parser integration; for the MVP render skeleton we
	// emit an empty payload with the report ID — enough for app.js to
	// wire localStorage keys and for the charts to render "empty"
	// banners without throwing.
	payload, _ := json.Marshal(map[string]any{
		"reportID": r.ReportID,
		"charts":   buildChartPayload(r),
	})
	v.DataPayload = template.JS(payload)

	return v
}

func buildChartPayload(r *model.Report) map[string]any {
	// In the MVP skeleton, parsers don't populate time-series data yet.
	// Return an empty-but-valid object so app.js' JSON.parse succeeds
	// and its chart-rendering loop finds no entries. Populated per
	// collector in US2-US4.
	return map[string]any{}
}

func flattenDiagnostics(c *model.Collection) []diagnosticView {
	var out []diagnosticView
	add := func(d model.Diagnostic) {
		out = append(out, diagnosticView{
			SourceFileDisplay: shortPath(d.SourceFile),
			Location:          d.Location,
			SeverityClass:     severityClass(d.Severity),
			SeverityLabel:     severityLabel(d.Severity),
			Message:           d.Message,
		})
	}
	for _, d := range c.Diagnostics {
		add(d)
	}
	for _, snap := range c.Snapshots {
		for _, suffix := range model.KnownSuffixes {
			sf, ok := snap.SourceFiles[suffix]
			if !ok {
				continue
			}
			for _, d := range sf.Diagnostics {
				add(d)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SeverityClass != out[j].SeverityClass {
			return severityRank(out[i].SeverityClass) > severityRank(out[j].SeverityClass)
		}
		if out[i].SourceFileDisplay != out[j].SourceFileDisplay {
			return out[i].SourceFileDisplay < out[j].SourceFileDisplay
		}
		return out[i].Location < out[j].Location
	})
	return out
}

func severityClass(s model.Severity) string {
	switch s {
	case model.SeverityInfo:
		return "info"
	case model.SeverityWarning:
		return "warn"
	case model.SeverityError:
		return "err"
	default:
		return "info"
	}
}

func severityLabel(s model.Severity) string {
	switch s {
	case model.SeverityInfo:
		return "info"
	case model.SeverityWarning:
		return "warn"
	case model.SeverityError:
		return "error"
	default:
		return "info"
	}
}

func severityRank(cls string) int {
	switch cls {
	case "err":
		return 3
	case "warn":
		return 2
	default:
		return 1
	}
}

func shortPath(p string) string {
	if p == "" {
		return ""
	}
	// Strip leading directory components so diagnostics aren't cluttered
	// with the full absolute path the user passed on the CLI.
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
