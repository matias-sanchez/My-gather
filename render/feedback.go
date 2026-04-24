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
	// GitHubURL is the base URL the client-side dialog opens on
	// submit. The dialog appends "&title=<enc>&body=<enc>" at click
	// time. Embedded as a build-time constant — not configurable at
	// runtime, not derived from environment or input data.
	GitHubURL string

	// Categories is the ordered, English-only list of triage buckets
	// shown in the dialog's category selector. Order is intentional:
	// most-common buckets first, "Other" last. The template ranges
	// over this slice by integer index so iteration order is
	// deterministic.
	Categories []string
}

// feedbackGitHubURL is the Ideas-category new-discussion URL for the
// project repo. The dialog's submit handler appends title and body
// query parameters; this constant is only the base.
const feedbackGitHubURL = "https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas"

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
		Categories: cats,
	}
}
