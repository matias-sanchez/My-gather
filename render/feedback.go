package render

// Report-feedback header control — see
// specs/002-report-feedback-button/plan.md and
// specs/002-report-feedback-button/contracts/ui.md.
//
// The view exposed here is intentionally a pair of build-time
// constants. The rendered markup for the button and dialog MUST be
// byte-identical across runs on the same input (Principle IV); the
// shape of this struct makes that easy to verify — two calls to
// BuildFeedbackView return the same value (see feedback_test.go).

// FeedbackView carries the static values the report's feedback header
// control and its dialog need at render time. Every field is a
// build-time constant so the rendered markup is deterministic.
type FeedbackView struct {
	// GitHubURL is the base URL the client-side dialog falls back to
	// when the Worker path is unreachable (spec 003, research R8).
	// The dialog appends "&title=<enc>&body=<enc>" at click time.
	// Embedded as a build-time constant — not configurable at
	// runtime, not derived from environment or input data.
	GitHubURL string

	// WorkerURL is the HTTPS endpoint of the feedback Cloudflare
	// Worker (spec 003). The client-side dialog POSTs the feedback
	// JSON payload here; on any Worker failure the dialog falls back
	// to GitHubURL. Constant at build time (research R7) so the
	// rendered markup stays byte-identical across runs
	// (Principle IV).
	WorkerURL string

	// Categories is the ordered, English-only list of triage buckets
	// shown in the dialog's category selector. Order is intentional:
	// most-common buckets first, "Other" last. The template ranges
	// over this slice by integer index so iteration order is
	// deterministic.
	Categories []string
}

// feedbackGitHubURL is the new-issue URL used by the legacy
// `window.open` fallback path (when the Worker is unavailable). The
// dialog's submit handler appends title and body query parameters; the
// labels=… pre-fill maps the fallback issue to the same triage labels
// the Worker applies on the happy path, so the two code paths land the
// user on the same venue (GitHub Issues) and with the same label set.
// Principle XIII: one conceptual destination per one user gesture.
const feedbackGitHubURL = "https://github.com/matias-sanchez/My-gather/issues/new?labels=user-feedback,needs-triage"

// feedbackWorkerURL is the public HTTPS endpoint of the feedback
// Cloudflare Worker (spec 003, research R7). The URL is a build-time
// constant so the Principle IV determinism guarantee is preserved and
// two renders of the same Collection produce byte-identical output.
const feedbackWorkerURL = "https://my-gather-feedback.mati-orfeo.workers.dev/feedback"

// feedbackCategories is the authoritative list of triage buckets.
// Copy-on-return in BuildFeedbackView so callers cannot mutate the
// package-level slice.
var feedbackCategories = []string{
	"UI",
	"Parser",
	"Advisor",
	"Other",
}

// BuildFeedbackView returns the static FeedbackView used by the
// report template. Determinism contract: two calls within a single
// binary build MUST return DeepEqual values. No clock reads, no
// randomness, no environment lookups.
func BuildFeedbackView() FeedbackView {
	cats := make([]string, len(feedbackCategories))
	copy(cats, feedbackCategories)
	return FeedbackView{
		GitHubURL:  feedbackGitHubURL,
		WorkerURL:  feedbackWorkerURL,
		Categories: cats,
	}
}
