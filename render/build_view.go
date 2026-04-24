package render

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	"github.com/matias-sanchez/My-gather/findings"
	"github.com/matias-sanchez/My-gather/model"
)

// buildView flattens the Report into the template-friendly shape.
//
// sigs is the per-VariablesSection.PerSnapshot signature cache computed
// once in buildReport (see NIT #27) and reused by buildNavigation —
// passing it in avoids a second hash pass here.
func buildView(r *model.Report, c *model.Collection, sigs []string) (*reportView, error) {
	v := &reportView{
		Title:              r.Title,
		Hostname:           c.Hostname,
		Version:            r.Version,
		GitCommit:          r.GitCommit,
		GeneratedAtDisplay: formatTimestamp(r.GeneratedAt),
		SnapshotCount:      len(c.Snapshots),
		Navigation:         r.Navigation,
		NavGroups:          groupNavigation(r.Navigation),
		EmbeddedCSS:        template.CSS(embeddedChartCSS + "\n" + embeddedAppCSS),
		EmbeddedChartJS:    template.JS(embeddedChartJS),
		EmbeddedAppJS:      template.JS(embeddedAppJS),
		LogoDataURI:        template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(embeddedLogoPNG)),
		MysqladminSelectID: "mysqladmin-select",
		Feedback:           BuildFeedbackView(),
	}

	if r.EnvironmentSection != nil {
		v.Environment = buildEnvironmentView(r)
		v.HasEnvironment = v.Environment.HasHost || v.Environment.HasMySQL
		// Badge only calls out degraded captures; a complete
		// environment (host + mysql) leaves the badge empty so the
		// header stays visually clean.
		switch {
		case v.Environment.HasHost && v.Environment.HasMySQL:
			v.EnvBadge = ""
		case v.Environment.HasHost:
			v.EnvBadge = "host only"
		case v.Environment.HasMySQL:
			v.EnvBadge = "mysql only"
		default:
			v.EnvBadge = "missing"
		}
	} else {
		v.EnvBadge = "missing"
	}

	if r.OSSection != nil {
		v.HasIostat = r.OSSection.Iostat != nil
		v.HasTop = r.OSSection.Top != nil
		v.HasVmstat = r.OSSection.Vmstat != nil
		v.HasMeminfo = r.OSSection.Meminfo != nil
		if v.HasIostat {
			v.IostatSummary = summariseIostat(r.OSSection.Iostat)
		}
		if v.HasTop {
			v.TopSummary = summariseTop(r.OSSection.Top)
		}
		if v.HasVmstat {
			v.VmstatSummary = summariseVmstat(r.OSSection.Vmstat)
		}
		if v.HasMeminfo {
			v.MeminfoSummary = summariseMeminfo(r.OSSection.Meminfo)
		}
		v.HasNetworkCounters = r.OSSection.NetCounters != nil
		v.HasNetworkSockets = r.OSSection.NetSockets != nil
		v.HasNetwork = v.HasNetworkCounters || v.HasNetworkSockets
		if v.HasNetwork {
			v.NetworkSummary = summariseNetwork(r.OSSection.NetCounters, r.OSSection.NetSockets)
		}
	}
	totalSnaps := 0
	if r.VariablesSection != nil {
		totalSnaps = len(r.VariablesSection.PerSnapshot)
		defaults, supportedVersions := loadMySQLDefaults()
		haveAny := false
		// keptVariableRuns is the single source of truth for dedup;
		// navigation and this body iterate the same runs so they can
		// never disagree on which snapshots were collapsed.
		var lastKeptMap map[string]string // name → value from previous kept panel
		for _, run := range keptVariableRuns(r.VariablesSection, sigs) {
			startSV := r.VariablesSection.PerSnapshot[run.StartIdx]
			if run.NilData {
				v.VariableSnapshots = append(v.VariableSnapshots, variableSnapshotView{
					DetailsID: variablesSnapshotID(run.StartIdx),
					Title:     fmt.Sprintf("Snapshot %s", startSV.SnapshotPrefix),
					Badge:     fmt.Sprintf("snap #%d", run.StartIdx+1),
				})
				lastKeptMap = nil
				continue
			}
			vv := variableSnapshotView{
				DetailsID: variablesSnapshotID(run.StartIdx),
				Title:     fmt.Sprintf("Snapshot %s", startSV.SnapshotPrefix),
				Badge:     fmt.Sprintf("snap #%d", run.StartIdx+1),
				Count:     len(startSV.Data.Entries),
				rangeLo:   run.StartIdx + 1,
				rangeHi:   run.EndIdx + 1,
			}
			// Pull the captured MySQL version out of this snapshot's
			// own Entries so lookups against mysql-defaults.json can
			// pick the right version column. pt-stalk writes the
			// `version` system variable into the -variables file; if
			// it's missing we pass an empty string and classifyVariable
			// falls back to "unknown" for everything it couldn't match
			// version-free.
			capturedVersion := ""
			for _, e := range startSV.Data.Entries {
				if e.Name == "version" {
					capturedVersion = e.Value
					break
				}
			}
			vv.Entries = make([]variableRowView, 0, len(startSV.Data.Entries))
			for _, e := range startSV.Data.Entries {
				st := classifyVariable(defaults, supportedVersions, capturedVersion, e.Name, e.Value)
				if st == "modified" {
					vv.ModifiedCount++
				}
				// Flag the row as Changed when (1) there IS a previous
				// kept panel, (2) this variable is NOT in the volatile
				// ignorelist, and (3) its value differs from (or was
				// absent in) the previous panel. The first kept panel
				// never highlights — nothing to compare to.
				changed := false
				if lastKeptMap != nil {
					if _, vol := volatileVariables[strings.ToLower(e.Name)]; !vol {
						prev, ok := lastKeptMap[e.Name]
						if !ok || prev != e.Value {
							changed = true
						}
					}
				}
				if changed {
					vv.ChangedCount++
				}
				vv.Entries = append(vv.Entries, variableRowView{
					Name:    e.Name,
					Value:   e.Value,
					Status:  st,
					Changed: changed,
				})
			}
			haveAny = true
			v.VariableSnapshots = append(v.VariableSnapshots, vv)
			// Snapshot the name→value map for the next kept comparison.
			lastKeptMap = make(map[string]string, len(startSV.Data.Entries))
			for _, e := range startSV.Data.Entries {
				lastKeptMap[e.Name] = e.Value
			}
		}
		v.HasVariables = haveAny
		// Derive the presentation RangeNote once per kept panel.
		// Single-snapshot runs and nil-Data entries leave it empty.
		for idx := range v.VariableSnapshots {
			vs := &v.VariableSnapshots[idx]
			if vs.rangeLo != 0 && vs.rangeLo != vs.rangeHi {
				vs.RangeNote = formatRangeNote(vs.rangeLo, vs.rangeHi)
			}
		}
	}
	if v.HasVariables {
		unique := len(v.VariableSnapshots)
		if unique < totalSnaps {
			v.VariablesBadge = fmt.Sprintf("%d unique of %d", unique, totalSnaps)
		} else {
			v.VariablesBadge = fmt.Sprintf("%d snapshots", unique)
		}
	} else {
		v.VariablesBadge = "missing"
	}

	if r.DBSection != nil {
		v.InnoDBMetrics = aggregateInnoDBMetrics(r.DBSection.InnoDBPerSnapshot)
		v.HasInnoDB = len(v.InnoDBMetrics) > 0
		v.HasMysqladmin = r.DBSection.Mysqladmin != nil
		v.HasProcesslist = r.DBSection.Processlist != nil
		if v.HasMysqladmin {
			v.MysqladminVariables = append(v.MysqladminVariables, r.DBSection.Mysqladmin.VariableNames...)
			v.MysqladminCount = len(r.DBSection.Mysqladmin.VariableNames)
		}
	}
	// Advisor: rule-based findings derived from the captured data.
	fs := findings.Analyze(r)
	v.Findings = buildFindingViews(fs)
	v.AdvisorCounts = summariseFindings(fs)
	v.HasFindings = len(v.Findings) > 0
	v.AdvisorBadge = advisorBadge(v.AdvisorCounts)

	// Build the embedded data payload. Per-chart series are populated
	// by US2-US4 parser integration; for the MVP render skeleton we
	// emit an empty payload with the report ID — enough for app.js to
	// wire localStorage keys and for the charts to render "empty"
	// banners without throwing.
	// The embedded payload drives every interactive feature (charts,
	// filters, localStorage keys). A marshal error here would ship an
	// HTML that looks valid but is silently non-interactive, so fail
	// loudly instead of emitting a broken blob.
	payload, err := json.Marshal(map[string]any{
		"reportID": r.ReportID,
		"charts":   buildChartPayload(r),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedded data payload: %w", err)
	}
	v.DataPayload = template.JS(payload)

	return v, nil
}
