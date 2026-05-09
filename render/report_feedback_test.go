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
	"encoding/json"
	"html"
	"regexp"
	"strconv"
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
	contract := renderedFeedbackContract(t, out)

	wantSubstrings := []string{
		`id="feedback-dialog"`,
		// Spec 021-feedback-author-field FR-001: the Author input
		// must be present in the dialog markup.
		`id="feedback-field-author"`,
		`id="feedback-field-title"`,
		`id="feedback-field-body"`,
		`id="feedback-field-category"`,
		// The submit button starts disabled until the client JS
		// validates a non-empty title and a non-empty author.
		`id="feedback-submit"`,
		`disabled`,
		// Fallback is hidden until the dialog's submit handler
		// detects a popup-blocker and reveals the manual link.
		`id="feedback-fallback"`,
		`hidden`,
	}
	for _, category := range contract.Categories {
		// Each category lands as <option value="X">X</option>; the
		// `>X<` text literal survives regardless of attribute order.
		wantSubstrings = append(wantSubstrings, `>`+category+`<`)
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

	// Spec 021-feedback-author-field FR-001: the Author input must
	// be marked HTML-required and carry the maxlength attribute
	// rendered from the canonical contract value, so the browser-
	// native length cap matches the worker's validation cap.
	// Extract the entire <input ...> opening tag (HTML attribute
	// order is not significant, so anchor on the element start and
	// match the whole tag rather than scanning forward from the id).
	authorTag := extractAuthorInputTag(t, out)
	if !feedbackAuthorRequiredRE.MatchString(authorTag) {
		t.Errorf("feedback-field-author must carry the `required` attribute; tag was %q", authorTag)
	}
	wantMaxlen := strconv.Itoa(render.BuildFeedbackView().AuthorMaxChars)
	gotMaxlen := feedbackAuthorMaxlenRE.FindStringSubmatch(authorTag)
	if gotMaxlen == nil {
		t.Errorf("feedback-field-author must carry a maxlength attribute; tag was %q", authorTag)
	} else if gotMaxlen[1] != wantMaxlen {
		t.Errorf("feedback-field-author maxlength = %q, want %q; tag was %q", gotMaxlen[1], wantMaxlen, authorTag)
	}
}

// feedbackAuthorInputRE captures the <input ...> opening tag whose
// id attribute is "feedback-field-author", regardless of where the
// id sits in the attribute list. The (?s) flag lets the tag span
// line breaks; [^>]* stops at the closing '>' before any other tag.
var feedbackAuthorInputRE = regexp.MustCompile(
	`(?s)<input\b[^>]*\bid="feedback-field-author"[^>]*>`,
)

// feedbackAuthorRequiredRE matches the boolean `required` attribute
// as a standalone token (not part of an unrelated word like
// "required-name"). HTML allows both bare `required` and
// `required=""`/`required="required"`.
var feedbackAuthorRequiredRE = regexp.MustCompile(
	`\brequired(?:\s|=|>|/)`,
)

// feedbackAuthorMaxlenRE captures the maxlength integer regardless
// of attribute position within the tag.
var feedbackAuthorMaxlenRE = regexp.MustCompile(
	`\bmaxlength="(\d+)"`,
)

// extractAuthorInputTag returns the full <input ...> opening tag
// for the Author field. Uses an HTML-attribute-order-insensitive
// regex anchored to the input element rather than scanning forward
// from the id substring (the previous approach silently missed
// attributes preceding `id` and broke when attribute order changed).
func extractAuthorInputTag(t *testing.T, out string) string {
	t.Helper()
	tag := feedbackAuthorInputRE.FindString(out)
	if tag == "" {
		t.Fatalf("feedback-field-author <input> tag not found")
	}
	return tag
}

// TestFeedbackAuthorAppearsBeforeTitle: spec 021-feedback-author-field
// FR-001 — the Author input must be positioned above the Title field
// in document order so users see the attribution control first.
func TestFeedbackAuthorAppearsBeforeTitle(t *testing.T) {
	out := renderFeedbackFixture(t)
	authorIdx := strings.Index(out, `id="feedback-field-author"`)
	titleIdx := strings.Index(out, `id="feedback-field-title"`)
	if authorIdx == -1 {
		t.Fatalf("feedback-field-author id not found")
	}
	if titleIdx == -1 {
		t.Fatalf("feedback-field-title id not found")
	}
	if authorIdx >= titleIdx {
		t.Errorf("Author input must precede Title input (author at %d, title at %d)", authorIdx, titleIdx)
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

var feedbackContractAttrRE = regexp.MustCompile(`data-feedback-contract="([^"]+)"`)

type feedbackContractForTest struct {
	GitHubURL  string   `json:"githubUrl"`
	WorkerURL  string   `json:"workerUrl"`
	Categories []string `json:"categories"`
}

func renderedFeedbackContract(t *testing.T, out string) feedbackContractForTest {
	t.Helper()
	m := feedbackContractAttrRE.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("rendered HTML missing data-feedback-contract")
	}
	var contract feedbackContractForTest
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &contract); err != nil {
		t.Fatalf("data-feedback-contract is not valid JSON: %v", err)
	}
	return contract
}

// TestFeedbackDialogHasCanonicalContract: spec
// 003-feedback-backend-worker -- the <dialog> must carry the
// canonical feedback contract with both endpoints. The contract is
// build-time data, so the rendered value is fixed and deterministic
// (Principle IV).
func TestFeedbackDialogHasCanonicalContract(t *testing.T) {
	out := renderFeedbackFixture(t)
	contract := renderedFeedbackContract(t, out)

	if contract.WorkerURL == "" {
		t.Fatal("feedback contract missing Worker URL")
	}
	if contract.GitHubURL == "" {
		t.Fatal("feedback contract missing GitHub URL")
	}
	if strings.Contains(out, "data-feedback-worker-url") || strings.Contains(out, "data-feedback-url") {
		t.Fatal("rendered HTML must use data-feedback-contract instead of legacy per-URL attrs")
	}
}

// TestFeedbackSuccessMarkupPresent: spec 003-feedback-backend-worker —
// the inline success state lives next to the form inside the dialog.
// Rendered hidden so the default first paint shows the form; the
// client JS un-hides it on a 200 response from the Worker and fills
// #feedback-success-link with the GitHub issue URL.
func TestFeedbackSuccessMarkupPresent(t *testing.T) {
	out := renderFeedbackFixture(t)

	wantSubstrings := []string{
		`id="feedback-success"`,
		`id="feedback-success-link"`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("rendered HTML missing feedback-success marker %q", s)
		}
	}

	// #feedback-success must ship with the `hidden` attribute on the
	// same element so the success panel doesn't flash on first paint.
	idx := strings.Index(out, `id="feedback-success"`)
	if idx == -1 {
		t.Fatalf("feedback-success id not found")
	}
	tagEnd := strings.Index(out[idx:], ">")
	if tagEnd == -1 || !strings.Contains(out[idx:idx+tagEnd], "hidden") {
		t.Errorf("feedback-success must carry the `hidden` attribute on first paint")
	}
}

// TestFeedbackCategoriesRenderInExactOrder: ui.md / feedback.go —
// the dialog's category selector must emit options in the exact order
// declared by the canonical feedback contract. The empty-value entry
// is the "—" placeholder rendered first so users can submit without
// picking a bucket; the rest come from FeedbackView.Categories in
// declaration order.
func TestFeedbackCategoriesRenderInExactOrder(t *testing.T) {
	out := renderFeedbackFixture(t)
	contract := renderedFeedbackContract(t, out)

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

	want := append([]string{""}, contract.Categories...)
	if len(got) != len(want) {
		t.Fatalf("unexpected option count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("option %d: got %q, want %q (full order: got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}
