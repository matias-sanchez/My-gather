# Phase 1 Data Model — Report Feedback Button

This feature introduces no persistent data. The only entities are transient runtime structures: a Go-side view struct and a browser-side draft state machine. Both are documented here because downstream tasks (`/speckit.tasks`) need concrete shapes to key on.

## Go-side (binary)

### `render/feedback.go`

```go
// FeedbackView carries the constants needed by the report's feedback
// header control and its dialog. Every field is a build-time constant
// so render output remains byte-identical across runs on the same
// input (Principle IV).
type FeedbackView struct {
    // GitHubURL is the base URL the client-side dialog opens on submit.
    // The dialog appends `&title=<enc>&body=<enc>` at click time.
    // Embedded as a build-time constant — not configurable at runtime.
    GitHubURL string

    // Categories is the ordered, English-only list of triage buckets
    // shown in the dialog's category selector. Order is intentional:
    // most-common bucket first, "Other" last.
    Categories []string
}

// BuildFeedbackView returns the static view used by report.html.tmpl.
// Determinism contract: the returned FeedbackView MUST be deeply equal
// across all calls within a single binary build. No clock reads, no
// random IDs, no environment lookups.
func BuildFeedbackView() FeedbackView
```

**Validation rules**:

- `GitHubURL` MUST equal `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas`.
- `Categories` MUST contain exactly `["UI", "Parser", "Advisor", "Other"]` in that order (verified by unit test).

**Integration**:

- `render/render.go` adds one field to its root `ViewModel`: `Feedback FeedbackView`.
- `report.html.tmpl` references `.Feedback.GitHubURL` and `{{range $i, $c := .Feedback.Categories}}` to emit the button and the dialog's `<select>` options.

## Browser-side (runtime, transient)

### Draft state machine (`render/assets/app.js`)

Conceptual state held in a single closure-scoped object:

```javascript
const draft = {
    // null before first open; reset to fresh object each dialog open.
    title:         "",           // <input> value
    body:          "",           // <textarea> value
    category:      null,         // null or one of the Categories strings
    imageBlob:     null,         // null or Blob (image/*)
    imagePreviewUrl: null,       // null or object URL for thumbnail
    voiceBlob:     null,         // null or Blob (audio/*)
    voicePreviewUrl: null,       // null or object URL for <audio src>
};
```

### Recording state machine

```
idle ──(click Record, getUserMedia ok)──▶ recording
idle ◀──(click Record, getUserMedia fail)── idle (error visible)

recording ──(click Stop)──▶ idle (voiceBlob set)
recording ──(120s cap)────▶ idle (voiceBlob set)
recording ──(dialog close)─▶ idle (stream stopped, no blob kept)
```

**Invariants**:
- Only one active MediaRecorder at a time. Starting a new recording while one is active is blocked at the UI (Record button is disabled during recording).
- `getUserMedia` stream tracks MUST be stopped (`track.stop()`) on both natural stop and dialog close, so the browser's red "recording" indicator goes away.

### Object URL lifecycle

- `imagePreviewUrl` and `voicePreviewUrl` MUST be revoked (`URL.revokeObjectURL`) when:
  - The attachment is removed via the Remove button.
  - A new paste/recording replaces an existing one.
  - The dialog closes (Escape / Cancel / backdrop click).
- Forgetting revocation is a memory leak, not a correctness bug — but the cleanup is explicit in the tasks.

### Submit-time URL shape

```text
https://github.com/matias-sanchez/My-gather/discussions/new
  ?category=ideas
  &title=<encodeURIComponent(draft.title)>
  &body=<encodeURIComponent(maybePrefixCategory(draft.body, draft.category))>
```

Where `maybePrefixCategory` is:

```javascript
function maybePrefixCategory(body, category) {
    if (!category) return body;
    return "> Category: " + category + "\n\n" + body;
}
```

### URL length guard

Before submit, compute `url.length` after encoding. If `url.length > 7500` (conservative budget under GitHub's ~8192-character practical cap), block submit and render an inline length-warning message. This check MUST run on every change to title, body, or category (debounced, but on every commit path).

## Persistence

None. No cookies, no localStorage, no IndexedDB, no server state, no analytics. The draft dies when the dialog closes, whether by submission, cancellation, or page unload. This is an explicit invariant from FR-022.
