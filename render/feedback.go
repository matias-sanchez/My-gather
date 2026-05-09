package render

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
)

// Report-feedback header control — see
// specs/002-report-feedback-button/plan.md and
// specs/002-report-feedback-button/contracts/ui.md.
//
// The view exposed here is intentionally built from one embedded
// contract. The rendered markup for the button and dialog MUST be
// byte-identical across runs on the same input (Principle IV); the
// shape of this struct makes that easy to verify — two calls to
// BuildFeedbackView return the same value (see feedback_test.go).

// FeedbackView carries the static values the report's feedback header
// control and its dialog need at render time. Every field is a
// build-time constant so the rendered markup is deterministic.
type FeedbackView struct {
	// Categories is the ordered, English-only list of triage buckets
	// shown in the dialog's category selector. Order is intentional:
	// most-common buckets first, "Other" last. The template ranges
	// over this slice by integer index so iteration order is
	// deterministic.
	Categories []string

	// ContractJSON is the compact canonical feedback contract exposed
	// to the browser. The client JS reads validation limits from this
	// value instead of keeping duplicate constants.
	ContractJSON string

	// AuthorMaxChars is the maximum length, in characters, of the
	// required Author display name in the dialog. Mirrored from the
	// canonical feedback contract so the rendered <input
	// maxlength="..."> attribute matches the worker's validation
	// limit (single source of truth, Principle XIII).
	AuthorMaxChars int
}

//go:embed assets/feedback-contract.json
var embeddedFeedbackContractJSON []byte

type feedbackContract struct {
	GitHubURL  string   `json:"githubUrl"`
	WorkerURL  string   `json:"workerUrl"`
	Categories []string `json:"categories"`
	Limits     struct {
		TitleMaxChars         int `json:"titleMaxChars"`
		BodyMaxBytes          int `json:"bodyMaxBytes"`
		ImageMaxBytes         int `json:"imageMaxBytes"`
		VoiceMaxBytes         int `json:"voiceMaxBytes"`
		ReportVersionMaxChars int `json:"reportVersionMaxChars"`
		LegacyURLMaxChars     int `json:"legacyUrlMaxChars"`
		WorkerTimeoutMS       int `json:"workerTimeoutMs"`
		RequestMaxBytes       int `json:"requestMaxBytes"`
		AuthorMaxChars        int `json:"authorMaxChars"`
	} `json:"limits"`
}

var canonicalFeedbackContract, canonicalFeedbackContractJSON = loadFeedbackContract()

func loadFeedbackContract() (feedbackContract, string) {
	var contract feedbackContract
	if err := json.Unmarshal(embeddedFeedbackContractJSON, &contract); err != nil {
		panic(fmt.Sprintf("render/assets/feedback-contract.json: malformed embedded JSON: %v", err))
	}
	if contract.GitHubURL == "" || contract.WorkerURL == "" || len(contract.Categories) == 0 {
		panic("render/assets/feedback-contract.json: missing required feedback contract values")
	}
	limits := contract.Limits
	if limits.TitleMaxChars <= 0 || limits.BodyMaxBytes <= 0 ||
		limits.ImageMaxBytes <= 0 || limits.VoiceMaxBytes <= 0 ||
		limits.ReportVersionMaxChars <= 0 || limits.LegacyURLMaxChars <= 0 ||
		limits.WorkerTimeoutMS <= 0 || limits.RequestMaxBytes <= 0 ||
		limits.AuthorMaxChars <= 0 {
		panic("render/assets/feedback-contract.json: feedback limits must be positive")
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, embeddedFeedbackContractJSON); err != nil {
		panic(fmt.Sprintf("render/assets/feedback-contract.json: compact embedded JSON: %v", err))
	}
	return contract, compact.String()
}

// BuildFeedbackView returns the static FeedbackView used by the
// report template. Determinism contract: two calls within a single
// binary build MUST return DeepEqual values. No clock reads, no
// randomness, no environment lookups.
func BuildFeedbackView() FeedbackView {
	cats := make([]string, len(canonicalFeedbackContract.Categories))
	copy(cats, canonicalFeedbackContract.Categories)
	return FeedbackView{
		Categories:     cats,
		ContractJSON:   canonicalFeedbackContractJSON,
		AuthorMaxChars: canonicalFeedbackContract.Limits.AuthorMaxChars,
	}
}
