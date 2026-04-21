package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"sort"
	"strings"
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

//go:embed assets/mysql-defaults.json
var embeddedMySQLDefaultsJSON []byte

//go:embed assets/mysqladmin-categories.json
var embeddedMysqladminCategoriesJSON []byte

type mysqladminCategoryDef struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	Matchers        []string `json:"matchers"`
	ExcludeMatchers []string `json:"exclude_matchers"`
	Members         []string `json:"members"`
}

var (
	mysqladminCategories     []mysqladminCategoryDef
	mysqladminCategoriesOnce bool
)

func loadMysqladminCategories() []mysqladminCategoryDef {
	if mysqladminCategoriesOnce {
		return mysqladminCategories
	}
	mysqladminCategoriesOnce = true
	var v struct {
		Categories []mysqladminCategoryDef `json:"categories"`
	}
	if err := json.Unmarshal(embeddedMysqladminCategoriesJSON, &v); err == nil {
		mysqladminCategories = v.Categories
	}
	return mysqladminCategories
}

// classifyMysqladminCategory returns the slugs of every category that
// claims `name`. Matching is case-insensitive on prefixes. Explicit
// `members` take precedence; `exclude_matchers` carve out a subset of
// a broader matcher (e.g., buffer-pool excludes the read/write
// subviews so they live in InnoDB Reads / Writes instead).
func classifyMysqladminCategory(cats []mysqladminCategoryDef, name string) []string {
	lower := strings.ToLower(name)
	var hits []string
	for _, c := range cats {
		// Members list — highest priority.
		matched := false
		for _, m := range c.Members {
			if strings.EqualFold(m, name) {
				matched = true
				break
			}
		}
		if !matched {
			for _, mp := range c.Matchers {
				if strings.HasPrefix(lower, strings.ToLower(mp)) {
					matched = true
					break
				}
			}
			if matched {
				for _, ex := range c.ExcludeMatchers {
					if strings.HasPrefix(lower, strings.ToLower(ex)) {
						matched = false
						break
					}
				}
			}
		}
		if matched {
			hits = append(hits, c.Key)
		}
	}
	return hits
}

// mysqlDefaults is the parsed default-values map, populated on first use.
var (
	mysqlDefaults     map[string]string
	mysqlDefaultsOnce bool
)

func loadMySQLDefaults() map[string]string {
	if mysqlDefaultsOnce {
		return mysqlDefaults
	}
	mysqlDefaultsOnce = true
	var v struct {
		Defaults map[string]string `json:"defaults"`
	}
	if err := json.Unmarshal(embeddedMySQLDefaultsJSON, &v); err == nil {
		mysqlDefaults = v.Defaults
	} else {
		mysqlDefaults = map[string]string{}
	}
	return mysqlDefaults
}

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

// groupNavigation flattens Report.Navigation into one navGroupView per
// Level-1 entry with its Level-2 children nested. Order is preserved.
func groupNavigation(nav []model.NavEntry) []navGroupView {
	var groups []navGroupView
	idxByID := map[string]int{}
	for _, e := range nav {
		if e.Level == 1 {
			groups = append(groups, navGroupView{ID: e.ID, Title: e.Title})
			idxByID[e.ID] = len(groups) - 1
		} else if e.Level == 2 {
			parentIdx, ok := idxByID[e.ParentID]
			if !ok {
				continue
			}
			groups[parentIdx].Children = append(groups[parentIdx].Children, navChildView{ID: e.ID, Title: e.Title})
		}
	}
	return groups
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
	NavGroups          []navGroupView

	EmbeddedCSS     template.CSS
	EmbeddedChartJS template.JS
	EmbeddedAppJS   template.JS
	DataPayload     template.JS // JSON, emitted inside <script type="application/json">

	OSBadge          string
	VariablesBadge   string
	DBBadge          string
	DiagnosticsBadge string

	// OS section payload
	HasIostat     bool
	HasTop        bool
	HasVmstat     bool
	IostatSummary *iostatSummaryView
	TopSummary    *topSummaryView
	VmstatSummary *vmstatSummaryView

	// Variables section payload
	HasVariables      bool
	VariableSnapshots []variableSnapshotView

	// DB section payload
	HasInnoDB           bool
	InnoDBSnapshots     []innoDBSnapshotView
	HasMysqladmin       bool
	MysqladminVariables []string
	MysqladminCount     int
	MysqladminSelectID  string
	HasProcesslist      bool

	// Diagnostics (flattened for template iteration).
	Diagnostics []diagnosticView
}

type variableSnapshotView struct {
	DetailsID    string
	Title        string
	Badge        string
	Count        int
	ModifiedCount int
	Entries      []variableRowView
}

type variableRowView struct {
	Name   string
	Value  string
	Status string // "default" | "modified" | "unknown"
}

type innoDBSnapshotView struct {
	Title         string
	Data          *model.InnodbStatusData
	PendingTotal  int
	AHISearchRate string
}

// navGroupView is the template-facing shape of one Level-1 nav entry
// plus its Level-2 children. Built from the flat Report.Navigation.
type navGroupView struct {
	ID       string
	Title    string
	Children []navChildView
}

type navChildView struct {
	ID    string
	Title string
}

type diagnosticView struct {
	SourceFileDisplay string
	Location          string
	SeverityClass     string
	SeverityLabel     string
	Message           string
}

type iostatSummaryView struct {
	PeakUtil        string
	PeakDevice      string
	PeakAqusz       string
	PeakAquszDevice string
	DeviceCount     int
	SampleCount     int
}

type topSummaryView struct {
	First       string
	FirstAvg    string
	Second      string
	SecondAvg   string
	Third       string
	ThirdAvg    string
	SampleCount int
}

type vmstatSummaryView struct {
	PeakRunqueue string
	PeakBlocked  string
	PeakIowait   string
	SampleCount  int
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
		NavGroups:          groupNavigation(r.Navigation),
		EmbeddedCSS:        template.CSS(embeddedChartCSS + "\n" + embeddedAppCSS),
		EmbeddedChartJS:    template.JS(embeddedChartJS),
		EmbeddedAppJS:      template.JS(embeddedAppJS),
		MysqladminSelectID: "mysqladmin-select",
	}

	if r.OSSection != nil {
		v.HasIostat = r.OSSection.Iostat != nil
		v.HasTop = r.OSSection.Top != nil
		v.HasVmstat = r.OSSection.Vmstat != nil
		if v.HasIostat {
			v.IostatSummary = summariseIostat(r.OSSection.Iostat)
		}
		if v.HasTop {
			v.TopSummary = summariseTop(r.OSSection.Top)
		}
		if v.HasVmstat {
			v.VmstatSummary = summariseVmstat(r.OSSection.Vmstat)
		}
	}
	presentOS := boolToInt(v.HasIostat) + boolToInt(v.HasTop) + boolToInt(v.HasVmstat)
	v.OSBadge = fmt.Sprintf("%d / 3 subviews", presentOS)

	if r.VariablesSection != nil {
		defaults := loadMySQLDefaults()
		haveAny := false
		for i, sv := range r.VariablesSection.PerSnapshot {
			vv := variableSnapshotView{
				DetailsID: variablesSnapshotID(i),
				Title:     fmt.Sprintf("Snapshot %s", sv.SnapshotPrefix),
				Badge:     fmt.Sprintf("snap #%d", i+1),
			}
			if sv.Data != nil {
				vv.Count = len(sv.Data.Entries)
				vv.Entries = make([]variableRowView, 0, len(sv.Data.Entries))
				for _, e := range sv.Data.Entries {
					st := classifyVariable(defaults, e.Name, e.Value)
					if st == "modified" {
						vv.ModifiedCount++
					}
					vv.Entries = append(vv.Entries, variableRowView{
						Name:   e.Name,
						Value:  e.Value,
						Status: st,
					})
				}
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
			v.MysqladminCount = len(r.DBSection.Mysqladmin.VariableNames)
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
	payload := map[string]any{}
	if r.OSSection != nil {
		if r.OSSection.Iostat != nil {
			payload["iostat"] = iostatChartPayload(r.OSSection.Iostat)
		}
		if r.OSSection.Top != nil {
			payload["top"] = topChartPayload(r.OSSection.Top)
		}
		if r.OSSection.Vmstat != nil {
			payload["vmstat"] = vmstatChartPayload(r.OSSection.Vmstat)
		}
	}
	if r.DBSection != nil {
		if r.DBSection.Mysqladmin != nil {
			payload["mysqladmin"] = mysqladminChartPayload(r.DBSection.Mysqladmin)
		}
		if r.DBSection.Processlist != nil {
			payload["processlist"] = processlistChartPayload(r.DBSection.Processlist)
		}
	}
	return payload
}

func mysqladminChartPayload(d *model.MysqladminData) map[string]any {
	timestamps := make([]float64, len(d.Timestamps))
	for i, t := range d.Timestamps {
		timestamps[i] = float64(t.Unix())
	}
	// app.js reads { variables, timestamps, deltas, isCounter, snapshotBoundaries, defaultVisible, categories, categoryMap }.
	// Replace NaN values with null for JSON encoding (math.NaN is not a
	// valid JSON number).
	deltas := map[string]any{}
	for name, vs := range d.Deltas {
		cleaned := make([]any, len(vs))
		for i, v := range vs {
			if math.IsNaN(v) {
				cleaned[i] = nil
			} else {
				cleaned[i] = v
			}
		}
		deltas[name] = cleaned
	}
	// Default visible: pick up to 5 well-known high-signal counters if
	// they exist; otherwise first 5 counter variables alphabetically.
	defaults := pickDefaultCounters(d)

	// Resolve per-variable category memberships once at render time so
	// the JS doesn't have to evaluate matchers in the browser.
	cats := loadMysqladminCategories()
	categoryMap := map[string][]string{}
	categoryCounts := map[string]int{}
	for _, name := range d.VariableNames {
		hits := classifyMysqladminCategory(cats, name)
		if len(hits) > 0 {
			categoryMap[name] = hits
			for _, h := range hits {
				categoryCounts[h]++
			}
		}
	}
	// Emit category metadata in declared order plus a count of matched
	// variables for each.
	type catView struct {
		Key         string `json:"key"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Count       int    `json:"count"`
	}
	categoryViews := make([]catView, 0, len(cats))
	for _, c := range cats {
		categoryViews = append(categoryViews, catView{
			Key:         c.Key,
			Label:       c.Label,
			Description: c.Description,
			Count:       categoryCounts[c.Key],
		})
	}
	return map[string]any{
		"variables":          d.VariableNames,
		"timestamps":         timestamps,
		"deltas":             deltas,
		"isCounter":          d.IsCounter,
		"snapshotBoundaries": d.SnapshotBoundaries,
		"defaultVisible":     defaults,
		"categories":         categoryViews,
		"categoryMap":        categoryMap,
	}
}

func pickDefaultCounters(d *model.MysqladminData) []string {
	preferred := []string{"Com_select", "Com_insert", "Com_update", "Questions", "Bytes_received", "Bytes_sent"}
	present := map[string]bool{}
	for _, n := range d.VariableNames {
		present[n] = true
	}
	var out []string
	for _, p := range preferred {
		if present[p] && d.IsCounter[p] {
			out = append(out, p)
			if len(out) >= 5 {
				return out
			}
		}
	}
	// Fallback: first 5 counters alphabetically.
	for _, n := range d.VariableNames {
		if len(out) >= 5 {
			break
		}
		if d.IsCounter[n] {
			already := false
			for _, x := range out {
				if x == n {
					already = true
					break
				}
			}
			if !already {
				out = append(out, n)
			}
		}
	}
	return out
}

func processlistChartPayload(d *model.ProcesslistData) map[string]any {
	if len(d.ThreadStateSamples) == 0 {
		return nil
	}
	timestamps := make([]float64, len(d.ThreadStateSamples))
	for i, s := range d.ThreadStateSamples {
		timestamps[i] = float64(s.Timestamp.Unix())
	}
	var series []map[string]any
	for _, state := range d.States {
		vals := make([]float64, len(d.ThreadStateSamples))
		for i, s := range d.ThreadStateSamples {
			vals[i] = float64(s.StateCounts[state])
		}
		series = append(series, map[string]any{
			"label":  state,
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps": timestamps,
		"series":     series,
	}
}

// chartTimestamps extracts a unified timestamp axis from an arbitrary
// primary series. Timestamps are rendered as unix seconds (float) for
// uPlot's time-scale.
func chartTimestamps(samples []model.Sample) []float64 {
	out := make([]float64, len(samples))
	for i, s := range samples {
		out[i] = float64(s.Timestamp.Unix())
	}
	return out
}

func iostatChartPayload(d *model.IostatData) map[string]any {
	if len(d.Devices) == 0 {
		return nil
	}
	// Use the first device's timestamps as the x-axis.
	timestamps := chartTimestamps(d.Devices[0].Utilization.Samples)
	var series []map[string]any
	// Emit %util for every device first, then avgqu-sz.
	for _, dev := range d.Devices {
		vals := make([]float64, len(dev.Utilization.Samples))
		for i, s := range dev.Utilization.Samples {
			vals[i] = s.Measurements["util_percent"]
		}
		series = append(series, map[string]any{
			"label":  dev.Device + " %util",
			"values": vals,
		})
	}
	for _, dev := range d.Devices {
		vals := make([]float64, len(dev.AvgQueueSize.Samples))
		for i, s := range dev.AvgQueueSize.Samples {
			vals[i] = s.Measurements["avgqu_sz"]
		}
		series = append(series, map[string]any{
			"label":  dev.Device + " aqu-sz",
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps": timestamps,
		"series":     series,
	}
}

func topChartPayload(d *model.TopData) map[string]any {
	if len(d.Top3ByAverage) == 0 {
		return nil
	}
	// Use the first top-3 process's timestamps as the x-axis.
	timestamps := chartTimestamps(d.Top3ByAverage[0].CPU.Samples)
	var series []map[string]any
	for _, ps := range d.Top3ByAverage {
		vals := make([]float64, len(ps.CPU.Samples))
		for i, s := range ps.CPU.Samples {
			vals[i] = s.Measurements["cpu_percent"]
		}
		series = append(series, map[string]any{
			"label":  fmt.Sprintf("%s (pid %d)", truncateCommand(ps.Command), ps.PID),
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps": timestamps,
		"series":     series,
	}
}

func vmstatChartPayload(d *model.VmstatData) map[string]any {
	if len(d.Series) == 0 {
		return nil
	}
	// Use the first series' timestamps.
	timestamps := chartTimestamps(d.Series[0].Samples)
	var series []map[string]any
	for _, s := range d.Series {
		vals := make([]float64, len(s.Samples))
		for i, sp := range s.Samples {
			vals[i] = sp.Measurements[s.Metric]
		}
		series = append(series, map[string]any{
			"label":  s.Metric,
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps": timestamps,
		"series":     series,
	}
}

func truncateCommand(s string) string {
	const max = 20
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func summariseIostat(d *model.IostatData) *iostatSummaryView {
	sum := &iostatSummaryView{DeviceCount: len(d.Devices)}
	if len(d.Devices) == 0 {
		return sum
	}
	var maxUtil, maxAqu float64
	var maxUtilDev, maxAquDev string
	sampleCount := 0
	for _, dev := range d.Devices {
		if len(dev.Utilization.Samples) > sampleCount {
			sampleCount = len(dev.Utilization.Samples)
		}
		for _, s := range dev.Utilization.Samples {
			if v := s.Measurements["util_percent"]; v > maxUtil {
				maxUtil = v
				maxUtilDev = dev.Device
			}
		}
		for _, s := range dev.AvgQueueSize.Samples {
			if v := s.Measurements["avgqu_sz"]; v > maxAqu {
				maxAqu = v
				maxAquDev = dev.Device
			}
		}
	}
	sum.PeakUtil = FormatFloat(maxUtil, 1) + "%"
	sum.PeakDevice = fallback(maxUtilDev, "–")
	sum.PeakAqusz = FormatFloat(maxAqu, 2)
	sum.PeakAquszDevice = fallback(maxAquDev, "–")
	sum.SampleCount = sampleCount
	return sum
}

func summariseTop(d *model.TopData) *topSummaryView {
	sum := &topSummaryView{}
	uniq := map[time.Time]struct{}{}
	for _, s := range d.ProcessSamples {
		uniq[s.Timestamp] = struct{}{}
	}
	sum.SampleCount = len(uniq)
	labels := make([]string, 0, 3)
	avgs := make([]string, 0, 3)
	for _, ps := range d.Top3ByAverage {
		var total float64
		for _, s := range ps.CPU.Samples {
			total += s.Measurements["cpu_percent"]
		}
		avg := 0.0
		if sum.SampleCount > 0 {
			avg = total / float64(sum.SampleCount)
		}
		labels = append(labels, truncateCommand(ps.Command)+" (pid "+fmt.Sprintf("%d", ps.PID)+")")
		avgs = append(avgs, FormatFloat(avg, 1))
	}
	if len(labels) > 0 {
		sum.First, sum.FirstAvg = labels[0], avgs[0]
	}
	if len(labels) > 1 {
		sum.Second, sum.SecondAvg = labels[1], avgs[1]
	}
	if len(labels) > 2 {
		sum.Third, sum.ThirdAvg = labels[2], avgs[2]
	}
	return sum
}

func summariseVmstat(d *model.VmstatData) *vmstatSummaryView {
	sum := &vmstatSummaryView{}
	peakForMetric := func(name string) float64 {
		for _, s := range d.Series {
			if s.Metric != name {
				continue
			}
			sum.SampleCount = maxInt(sum.SampleCount, len(s.Samples))
			var peak float64
			for _, sp := range s.Samples {
				if v := sp.Measurements[name]; v > peak {
					peak = v
				}
			}
			return peak
		}
		return 0
	}
	sum.PeakRunqueue = FormatFloat(peakForMetric("runqueue"), 0)
	sum.PeakBlocked = FormatFloat(peakForMetric("blocked"), 0)
	sum.PeakIowait = FormatFloat(peakForMetric("cpu_iowait"), 0)
	return sum
}

func fallback(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// classifyVariable compares a captured variable value against the
// documented compiled-in default. Returns:
//   - "default"  — value matches the documented default
//   - "modified" — value differs from the documented default
//   - "unknown"  — no default is documented for this variable
//
// Matching is tolerant: whitespace-trimmed, case-insensitive, and
// comma-separated values (e.g. sql_mode, tls_version) compared as
// sets so member order does not flag a default as modified.
func classifyVariable(defaults map[string]string, name, observed string) string {
	def, ok := defaults[name]
	if !ok {
		return "unknown"
	}
	if normalisedEqual(def, observed) {
		return "default"
	}
	if commaSetsEqual(def, observed) {
		return "default"
	}
	return "modified"
}

func normalisedEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func commaSetsEqual(a, b string) bool {
	if !strings.Contains(a, ",") && !strings.Contains(b, ",") {
		return false
	}
	split := func(s string) []string {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			out = append(out, strings.ToLower(p))
		}
		sort.Strings(out)
		return out
	}
	xs, ys := split(a), split(b)
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
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
