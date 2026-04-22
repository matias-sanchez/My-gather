package render_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/matias-sanchez/My-gather/render"
)

// TestDeterminismStress: T082 / SC-003 / Principle IV.
//
// SC-003 requires that a rendered report be byte-identical across
// re-runs with the same inputs and the same `GeneratedAt`. The
// render_test.go's TestRenderDeterministic already asserts this for
// one re-render; T082 strengthens the guarantee with a 10-iteration
// stress loop against the committed example2 fixture.
//
// The rationale: a flaky determinism failure (map iteration order
// leaking, time.Now() leaking, a goroutine-scheduling-dependent
// formatter) is easy to miss in a single re-render comparison. Ten
// iterations give the harness enough surface area to surface a
// probability-1/N bug if one slipped in.
//
// The test renders against the full example2 fixture (not the
// minimal hand-built Collection) because the real fixture exercises
// every code path — charts, mysqladmin deltas, processlist thread-
// state buckets — whereas a hand-built minimal Collection misses
// the parts of the renderer most likely to harbour non-determinism.
//
// The iteration count (10) is a judgement call: large enough to be
// useful, small enough to stay under one second on a CI runner. If
// future work wants a much larger stress count it should live in a
// `-long` build tag rather than the default suite.
func TestDeterminismStress(t *testing.T) {
	const iterations = 10

	c := discoverExample2(t)
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}

	var firstHash string
	for i := 0; i < iterations; i++ {
		var buf bytes.Buffer
		if err := render.Render(&buf, c, opts); err != nil {
			t.Fatalf("iteration %d: Render: %v", i, err)
		}
		sum := sha256.Sum256(buf.Bytes())
		h := hex.EncodeToString(sum[:])
		if i == 0 {
			firstHash = h
			continue
		}
		if h != firstHash {
			// Dump the byte-length delta as the first diagnostic —
			// a size mismatch on a deterministic fixture is almost
			// always a map-iteration-order leak or a format string
			// that pulled in time.Now() without honouring
			// GeneratedAt. The full HTML is too large to print;
			// callers debug via diff of two manual renders.
			t.Fatalf("iteration %d: SHA-256(output)=%s differs from iteration 0's %s; SC-003 determinism violated across %d renders with identical inputs + opts",
				i, h, firstHash, iterations)
		}
	}
}
