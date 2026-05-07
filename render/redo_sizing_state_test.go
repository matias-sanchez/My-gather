package render

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// redoSizingPanelTemplate is a verbatim copy of the redo-sizing panel
// fragment from render/templates/db.html.tmpl. Tests use it to assert
// the State value drives the EXPECTED template branch end-to-end —
// the production template is the consumer of computeRedoSizing's
// State priority order, so the assertion must exercise the same
// branch structure (Principle XIII).
const redoSizingPanelTemplate = `` +
	`{{- with .RedoSizing }}` +
	`<div class="redo-sizing">` +
	`{{- if eq .State "config_missing" }}` +
	`<p class="v">unavailable</p>` +
	`<p class="sub">Redo configuration variables not present in this capture.</p>` +
	`{{- else }}` +
	`<p class="v">{{.ConfiguredText}}</p>` +
	`<p class="sub">configured redo space — source: <code>{{.ConfigSource}}</code></p>` +
	`{{- end }}` +
	`{{- if eq .State "rate_unavailable" }}` +
	`<p class="stats">Observed write rate: unavailable (no <code>Innodb_os_log_written</code> samples)</p>` +
	`{{- else if eq .State "no_writes" }}` +
	`<p class="stats">Observed average write rate: <span>{{.ObservedRateText}}</span> · <span>{{.ObservedRatePerMinText}}</span></p>` +
	`<p class="stats">No observed redo writes during the capture — coverage and recommendations are not applicable.</p>` +
	`{{- else }}` +
	`<p class="stats">Observed average write rate: <span>{{.ObservedRateText}}</span> · <span>{{.ObservedRatePerMinText}}</span></p>` +
	`<p class="stats">Peak write rate ({{.PeakWindowLabel}} rolling): <span>{{.PeakRateText}}</span></p>` +
	`{{- if eq .State "ok" }}` +
	`<p class="stats">Coverage at peak: <span>{{.CoverageText}}</span></p>` +
	`{{- end }}` +
	`{{- end }}` +
	`</div>` +
	`{{- end }}`

// renderRedoSizingPanel executes the panel-fragment template against
// the supplied view and returns the rendered HTML so tests can assert
// on the user-visible output.
func renderRedoSizingPanel(t *testing.T, v *redoSizingView) string {
	t.Helper()
	tmpl, err := template.New("redo").Parse(redoSizingPanelTemplate)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ RedoSizing *redoSizingView }{v}); err != nil {
		t.Fatalf("execute template: %v", err)
	}
	return buf.String()
}

// TestComputeRedoSizing_DualMissing_PrefersRateUnavailable is the
// regression test for the Codex P2 finding at render/redo_sizing.go:145
// (PR #58 wave 5). When BOTH Innodb_os_log_written AND the redo
// configuration variables are absent, the canonical State priority
// (rate_unavailable > config_missing > no_writes > ok) requires the
// panel to render the explicit unavailable message rather than the
// generic rate rows with empty placeholder text such as
// "Peak write rate ( rolling)". The previous implementation overwrote
// State from "rate_unavailable" to "config_missing" in this dual-missing
// case, dropping the operator into the misleading fallback branch and
// regressing the FR-009 graceful-degradation behavior (Principle III).
func TestComputeRedoSizing_DualMissing_PrefersRateUnavailable(t *testing.T) {
	// Both variables AND counter are absent (dual-missing capture).
	r := makeRedoReport(t, nil, nil, 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "rate_unavailable" {
		t.Fatalf("State = %q, want rate_unavailable "+
			"(rate_unavailable must take priority over config_missing "+
			"when both degradation conditions hold)", v.State)
	}
	if v.ObservedRateText != "unavailable" {
		t.Fatalf("ObservedRateText = %q, want %q", v.ObservedRateText, "unavailable")
	}
	if v.PeakRateText != "unavailable" {
		t.Fatalf("PeakRateText = %q, want %q", v.PeakRateText, "unavailable")
	}
	if v.CoverageText != "n/a" {
		t.Fatalf("CoverageText = %q, want n/a", v.CoverageText)
	}
	if v.Recommended15MinText != "n/a" || v.Recommended1HourText != "n/a" {
		t.Fatalf("recommendations should be n/a, got %q / %q",
			v.Recommended15MinText, v.Recommended1HourText)
	}

	// Render the panel fragment and assert the misleading
	// fallback-branch text never appears. The bad output is recognizable
	// because PeakWindowLabel is empty in the rate_unavailable state,
	// so the template's "Peak write rate ({{.PeakWindowLabel}} rolling)"
	// line would render as the literal "Peak write rate ( rolling)"
	// if State were ever overwritten to config_missing.
	html := renderRedoSizingPanel(t, v)
	if strings.Contains(html, "Peak write rate ( rolling)") {
		t.Fatalf("rendered panel contains the misleading empty-window "+
			"placeholder %q; rate_unavailable state must not fall "+
			"through to the generic rate-rows branch:\n%s",
			"Peak write rate ( rolling)", html)
	}
	if strings.Contains(html, "Peak write rate (") {
		t.Fatalf("rendered panel contains a Peak-write-rate row at all "+
			"in the rate_unavailable state; only the explicit "+
			"unavailable message should render:\n%s", html)
	}
	if !strings.Contains(html, "Observed write rate: unavailable") {
		t.Fatalf("rendered panel missing the explicit "+
			"unavailable message required by FR-009:\n%s", html)
	}
}

// TestComputeRedoSizing_StatePriorityIsCanonical exercises every
// (rate present/absent) x (config present/absent) x (writes/no writes)
// combination and asserts that deriveRedoSizingState returns the
// highest-priority State per the canonical priority order documented
// in specs/019-redo-log-sizing-panel/contracts/redo-sizing-panel.md
// (rate_unavailable > config_missing > no_writes > ok). This is the
// belt-and-braces guard against a future branch silently overwriting
// State after the helper returns (Principle XIII).
func TestComputeRedoSizing_StatePriorityIsCanonical(t *testing.T) {
	cases := []struct {
		name       string
		rateOK     bool
		anyWrites  bool
		configured float64
		wantState  string
	}{
		{
			name:       "rate_unavailable_dominates_all",
			rateOK:     false,
			anyWrites:  false,
			configured: 0,
			wantState:  "rate_unavailable",
		},
		{
			name:       "rate_unavailable_dominates_config_missing",
			rateOK:     false,
			anyWrites:  true,
			configured: 0,
			wantState:  "rate_unavailable",
		},
		{
			name:       "rate_unavailable_with_config_present",
			rateOK:     false,
			anyWrites:  true,
			configured: 4 << 30,
			wantState:  "rate_unavailable",
		},
		{
			name:       "config_missing_dominates_no_writes",
			rateOK:     true,
			anyWrites:  false,
			configured: 0,
			wantState:  "config_missing",
		},
		{
			name:       "config_missing_with_writes",
			rateOK:     true,
			anyWrites:  true,
			configured: 0,
			wantState:  "config_missing",
		},
		{
			name:       "no_writes_with_config_present",
			rateOK:     true,
			anyWrites:  false,
			configured: 4 << 30,
			wantState:  "no_writes",
		},
		{
			name:       "ok_with_writes_and_config",
			rateOK:     true,
			anyWrites:  true,
			configured: 4 << 30,
			wantState:  "ok",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveRedoSizingState(tc.rateOK, tc.anyWrites, tc.configured)
			if got != tc.wantState {
				t.Fatalf("deriveRedoSizingState(rateOK=%v, anyWrites=%v, "+
					"configured=%v) = %q, want %q",
					tc.rateOK, tc.anyWrites, tc.configured, got, tc.wantState)
			}
		})
	}
}
