package render_test

// Template-level integration tests for the report-feedback button and
// its dialog (spec 002-report-feedback-button). These complement the
// unit tests in render/feedback_test.go — that file covers the static
// view builder (BuildFeedbackView); this file renders the full report
// through render.Render and asserts the DOM contract from
// specs/002-report-feedback-button/contracts/ui.md lands in the HTML
// byte-for-byte, and does so deterministically (Principle IV).
//
// Behaviour tests (dialog open/close, submit wiring, recording) are
// intentionally out of scope: those live in the Ola 3 Quickstart
// manual walkthrough.

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/render"
)

// renderFeedbackFixture runs the full render pipeline on the minimal
// empty-but-valid Collection fixture shared with render_test.go. The
// fixture + RenderOptions pair is the same one TestRenderEmptyReport
// uses, which keeps the feedback-markup assertions aligned with the
// baseline determinism guarantee the repo already enforces.
func renderFeedbackFixture(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, minimalCollection(), opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return buf.String()
}

// TestFeedbackButtonMarkupPresent: ui.md — the header-level
// Report-feedback trigger must expose the stable id, the ARIA
// popup/controls wiring the JS hooks into, and the human-readable
// label.
func TestFeedbackButtonMarkupPresent(t *testing.T) {
	out := renderFeedbackFixture(t)

	wantSubstrings := []string{
		`id="feedback-open"`,
		`aria-haspopup="dialog"`,
		`aria-controls="feedback-dialog"`,
		`Report feedback`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("rendered HTML missing feedback-button marker %q", s)
		}
	}
}

// TestFeedbackDialogMarkupPresent: ui.md — the <dialog> carrying the
// feedback form must land in the report with every field id the
// client JS targets, the submit button initially disabled, the
// fallback paragraph initially hidden, and every category value
// emitted as an <option>.
func TestFeedbackDialogMarkupPresent(t *testing.T) {
	out := renderFeedbackFixture(t)

	wantSubstrings := []string{
		`id="feedback-dialog"`,
		`id="feedback-field-title"`,
		`id="feedback-field-body"`,
		`id="feedback-field-category"`,
		// The submit button starts disabled until the client JS
		// validates a non-empty title.
		`id="feedback-submit"`,
		`disabled`,
		// Fallback is hidden until the dialog's submit handler
		// detects a popup-blocker and reveals the manual link.
		`id="feedback-fallback"`,
		`hidden`,
		// Each category lands as <option value="X">X</option>; the
		// `>X<` text literal survives regardless of attribute order.
		`>UI<`,
		`>Parser<`,
		`>Advisor<`,
		`>Other<`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("rendered HTML missing feedback-dialog marker %q", s)
		}
	}

	// Tighter cross-check: the submit button and fallback must each
	// carry the attribute in the same element, not just somewhere in
	// the document. This guards against the template accidentally
	// dropping `disabled` / `hidden` while leaving the id intact.
	submitIdx := strings.Index(out, `id="feedback-submit"`)
	if submitIdx == -1 {
		t.Fatalf("feedback-submit id not found")
	}
	submitTagEnd := strings.Index(out[submitIdx:], ">")
	if submitTagEnd == -1 || !strings.Contains(out[submitIdx:submitIdx+submitTagEnd], "disabled") {
		t.Errorf("feedback-submit must carry the `disabled` attribute on first paint")
	}
	fallbackIdx := strings.Index(out, `id="feedback-fallback"`)
	if fallbackIdx == -1 {
		t.Fatalf("feedback-fallback id not found")
	}
	fallbackTagEnd := strings.Index(out[fallbackIdx:], ">")
	if fallbackTagEnd == -1 || !strings.Contains(out[fallbackIdx:fallbackIdx+fallbackTagEnd], "hidden") {
		t.Errorf("feedback-fallback must carry the `hidden` attribute on first paint")
	}
}

// TestFeedbackMarkupIsDeterministic: Principle IV — two renders with
// the same Collection and the same RenderOptions must produce
// byte-identical output end-to-end across the feedback markup. The
// top-level determinism test in render_test.go already covers the
// whole report, but re-asserting here keeps the feedback feature's
// determinism guarantee co-located with its other contract tests so
// a regression surfaces from either angle.
func TestFeedbackMarkupIsDeterministic(t *testing.T) {
	c := minimalCollection()
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}

	var a, b bytes.Buffer
	if err := render.Render(&a, c, opts); err != nil {
		t.Fatalf("render a: %v", err)
	}
	if err := render.Render(&b, c, opts); err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("render is not deterministic across runs (len %d vs %d)", a.Len(), b.Len())
	}
}

// feedbackCategorySelectRE captures the contents of the
// <select id="feedback-field-category">…</select> region so the test
// can inspect just that slice of the document without being coupled
// to attribute ordering elsewhere.
var feedbackCategorySelectRE = regexp.MustCompile(
	`(?s)<select\s+id="feedback-field-category"[^>]*>(.*?)</select>`,
)

// feedbackOptionValueRE matches <option value="X">… with single or
// double quotes and captures the value. Ordering of matches in the
// slice reflects document order, which is the determinism contract.
var feedbackOptionValueRE = regexp.MustCompile(`<option\s+value="([^"]*)"`)

// TestFeedbackCategoriesRenderInExactOrder: ui.md / feedback.go —
// the dialog's category selector must emit options in the exact order
// ["", "UI", "Parser", "Advisor", "Other"]. The empty-value entry is
// the "—" placeholder rendered first so users can submit without
// picking a bucket; the rest come from FeedbackView.Categories in
// declaration order.
func TestFeedbackCategoriesRenderInExactOrder(t *testing.T) {
	out := renderFeedbackFixture(t)

	selectMatch := feedbackCategorySelectRE.FindStringSubmatch(out)
	if selectMatch == nil {
		t.Fatalf("could not locate <select id=\"feedback-field-category\"> region in output")
	}
	region := selectMatch[1]

	optionMatches := feedbackOptionValueRE.FindAllStringSubmatch(region, -1)
	got := make([]string, 0, len(optionMatches))
	for _, m := range optionMatches {
		got = append(got, m[1])
	}

	want := []string{"", "UI", "Parser", "Advisor", "Other"}
	if len(got) != len(want) {
		t.Fatalf("unexpected option count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("option %d: got %q, want %q (full order: got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}
