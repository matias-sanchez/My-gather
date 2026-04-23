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
	}
	totalSnaps := 0
	if r.VariablesSection != nil {
		totalSnaps = len(r.VariablesSection.PerSnapshot)
		defaults := loadMySQLDefaults()
		haveAny := false
		// Dedup adjacent identical snapshots. For each captured
		// snapshot compute a signature over the (name, value) pairs —
		// skipping volatile variables that drift without meaningful
		// change — and compare against the last KEPT snapshot. If
		// identical, extend that snapshot's range; otherwise emit a
		// new view. Nil-Data snapshots emit their own entry so
		// partial captures remain visible.
		lastKept := -1
		var lastSig string
		var lastKeptMap map[string]string // name → value from previous kept snapshot
		for i, sv := range r.VariablesSection.PerSnapshot {
			if sv.Data == nil {
				v.VariableSnapshots = append(v.VariableSnapshots, variableSnapshotView{
					DetailsID: variablesSnapshotID(i),
					Title:     fmt.Sprintf("Snapshot %s", sv.SnapshotPrefix),
					Badge:     fmt.Sprintf("snap #%d", i+1),
				})
				lastKept = -1
				lastSig = ""
				lastKeptMap = nil
				continue
			}
			sig := sigs[i]
			if lastKept >= 0 && sig == lastSig {
				// Identical to the last kept snapshot — extend its range.
				extendRange(&v.VariableSnapshots[lastKept], i+1)
				continue
			}
			vv := variableSnapshotView{
				DetailsID: variablesSnapshotID(i),
				Title:     fmt.Sprintf("Snapshot %s", sv.SnapshotPrefix),
				Badge:     fmt.Sprintf("snap #%d", i+1),
				Count:     len(sv.Data.Entries),
			}
			vv.Entries = make([]variableRowView, 0, len(sv.Data.Entries))
			for _, e := range sv.Data.Entries {
				st := classifyVariable(defaults, e.Name, e.Value)
				if st == "modified" {
					vv.ModifiedCount++
				}
				// Flag the row as Changed when (1) there IS a previous
				// kept snapshot, (2) this variable is NOT in the
				// volatile ignorelist, and (3) its value differs from
				// (or was absent in) the previous snapshot. The first
				// kept panel never highlights — nothing to compare to.
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
			lastKept = len(v.VariableSnapshots) - 1
			lastSig = sig
			// Snapshot the name→value map for the next kept comparison.
			lastKeptMap = make(map[string]string, len(sv.Data.Entries))
			for _, e := range sv.Data.Entries {
				lastKeptMap[e.Name] = e.Value
			}
			// Seed the range tracker with the current snapshot number
			// so extendRange can grow it later.
			v.VariableSnapshots[lastKept].RangeNote = formatRangeNote(i+1, i+1)
		}
		v.HasVariables = haveAny
		// RangeNote is only informative when the kept snapshot
		// represents a range; clear the single-snapshot notes.
		for idx := range v.VariableSnapshots {
			if rangeIsSingle(v.VariableSnapshots[idx].RangeNote) {
				v.VariableSnapshots[idx].RangeNote = ""
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
