package parse

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// makePtStalkSnapshot creates one timestamped pt-stalk file inside dir
// so dir satisfies LooksLikePtStalkRoot. The exact suffix and contents
// do not matter for discovery; only the filename shape does.
func makePtStalkSnapshot(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	name := filepath.Join(dir, "2026_05_08_12_00_00-mysqladmin")
	if err := os.WriteFile(name, []byte("Uptime: 1 Threads: 1\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// resolvedAbs returns the absolute, symlink-resolved form of p, which
// is the shape FindPtStalkRoot returns. On macOS, t.TempDir() lives
// under /var/folders, which is itself a symlink to /private/var/...,
// so test expectations must run through EvalSymlinks too or paths
// will not compare equal even when logically identical.
func resolvedAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs %s: %v", p, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatalf("evalsymlinks %s: %v", abs, err)
	}
	return resolved
}

func TestFindPtStalkRoot_TopLevel(t *testing.T) {
	tmp := t.TempDir()
	makePtStalkSnapshot(t, tmp)

	got, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := resolvedAbs(t, tmp)
	if got != want {
		t.Fatalf("root mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestFindPtStalkRoot_NestedSingle(t *testing.T) {
	tmp := t.TempDir()
	// 6 directories below tmp: a/b/c/d/e/f
	deep := filepath.Join(tmp, "a", "b", "c", "d", "e", "f")
	makePtStalkSnapshot(t, deep)

	got, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := resolvedAbs(t, deep)
	if got != want {
		t.Fatalf("root mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestFindPtStalkRoot_HiddenDirSkipped(t *testing.T) {
	tmp := t.TempDir()
	hidden := filepath.Join(tmp, ".cache", "pt", "host")
	makePtStalkSnapshot(t, hidden)

	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("want ErrNotAPtStalkDir, got %v", err)
	}
}

func TestFindPtStalkRoot_DepthCap(t *testing.T) {
	tmp := t.TempDir()
	// 12 directories below tmp; with default MaxDepth=8 the root at
	// depth 12 is unreachable.
	parts := []string{tmp, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	deep := filepath.Join(parts...)
	makePtStalkSnapshot(t, deep)

	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("want ErrNotAPtStalkDir, got %v", err)
	}
}

func TestFindPtStalkRoot_SymlinkNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin or developer mode on Windows")
	}
	tmp := t.TempDir()

	// Create the actual capture outside the input tree, then symlink
	// it from inside.
	external := t.TempDir()
	makePtStalkSnapshot(t, filepath.Join(external, "host"))

	link := filepath.Join(tmp, "indirect")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("want ErrNotAPtStalkDir (symlinks should not be followed), got %v", err)
	}
}

func TestFindPtStalkRoot_PermDeniedTolerated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permissions model differs; skip")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root - chmod 0 is not enforced")
	}
	tmp := t.TempDir()

	// One readable subdirectory containing the capture.
	readable := filepath.Join(tmp, "readable", "host")
	makePtStalkSnapshot(t, readable)

	// One unreadable subdirectory peer.
	unreadable := filepath.Join(tmp, "unreadable")
	if err := os.MkdirAll(unreadable, 0o700); err != nil {
		t.Fatalf("mkdir unreadable: %v", err)
	}
	// Put something inside it so a walk that descended would have to
	// list it; the listing is what fails when we strip permissions.
	if err := os.WriteFile(filepath.Join(unreadable, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write child: %v", err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod 0: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) })

	got, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if err != nil {
		t.Fatalf("unreadable subdir aborted walk: %v", err)
	}
	wantAbs := resolvedAbs(t, readable)
	if got != wantAbs {
		t.Fatalf("root mismatch:\n got=%s\nwant=%s", got, wantAbs)
	}
}

func TestFindPtStalkRoot_NestedMultiple(t *testing.T) {
	tmp := t.TempDir()
	hostA := filepath.Join(tmp, "alpha", "host")
	hostB := filepath.Join(tmp, "beta", "host")
	makePtStalkSnapshot(t, hostA)
	makePtStalkSnapshot(t, hostB)

	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	var mpe *MultiplePtStalkRootsError
	if !errors.As(err, &mpe) {
		t.Fatalf("want *MultiplePtStalkRootsError, got %T: %v", err, err)
	}
	if len(mpe.Roots) != 2 {
		t.Fatalf("want 2 roots, got %d: %v", len(mpe.Roots), mpe.Roots)
	}
	if !sort.StringsAreSorted(mpe.Roots) {
		t.Fatalf("roots are not lexically sorted: %v", mpe.Roots)
	}
	wantA := resolvedAbs(t, hostA)
	wantB := resolvedAbs(t, hostB)
	if mpe.Roots[0] != wantA || mpe.Roots[1] != wantB {
		t.Fatalf("roots mismatch:\n got=%v\nwant=[%s %s]", mpe.Roots, wantA, wantB)
	}
}

func TestFindPtStalkRoot_Deterministic(t *testing.T) {
	tmp := t.TempDir()
	for _, p := range []string{"alpha/host", "beta/host", "gamma/host"} {
		makePtStalkSnapshot(t, filepath.Join(tmp, p))
	}

	_, err1 := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	_, err2 := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if err1 == nil || err2 == nil {
		t.Fatalf("expected multi-root error on both invocations: err1=%v err2=%v", err1, err2)
	}
	if err1.Error() != err2.Error() {
		t.Fatalf("non-deterministic error message:\nrun1=%q\nrun2=%q", err1.Error(), err2.Error())
	}
}

func TestFindPtStalkRoot_NoRoot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "unrelated", "files"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	if !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("want ErrNotAPtStalkDir, got %v", err)
	}
}

func TestFindPtStalkRoot_RejectsNegativeOptions(t *testing.T) {
	tmp := t.TempDir()
	if _, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{MaxDepth: -1}); err == nil {
		t.Fatal("want error for negative MaxDepth, got nil")
	}
	if _, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{MaxEntries: -1}); err == nil {
		t.Fatal("want error for negative MaxEntries, got nil")
	}
}

func TestFindPtStalkRoot_NotADirectory(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := FindPtStalkRoot(context.Background(), f, FindPtStalkRootOptions{})
	var pe *PathError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PathError, got %T: %v", err, err)
	}
}

func TestMultiplePtStalkRootsError_MessageFormat(t *testing.T) {
	mpe := &MultiplePtStalkRootsError{Roots: []string{"/a/host1", "/b/host2"}}
	msg := mpe.Error()
	wantPrefix := "multiple pt-stalk roots found (2):"
	if !strings.HasPrefix(msg, wantPrefix) {
		t.Errorf("message prefix:\n got=%q\nwant prefix=%q", msg, wantPrefix)
	}
	if !strings.Contains(msg, "/a/host1") || !strings.Contains(msg, "/b/host2") {
		t.Errorf("message missing root paths:\n%s", msg)
	}
	if !strings.Contains(msg, "re-run pointing at one of these paths") {
		t.Errorf("message missing operator hint:\n%s", msg)
	}
}

// TestFindPtStalkRoot_IncludeHiddenFindsRootUnderDotDir guards the
// Codex round-5 regression: archive inputs route through the canonical
// walker and must preserve the pre-feature unbounded-walk behaviour
// of findExtractedPtStalkRoot. With opts.IncludeHidden=true a pt-stalk
// root nested under a hidden-named subdirectory (e.g. .cache/pt/...)
// is still discovered, matching the original archive-walker semantics.
// The directory-input default (IncludeHidden=false) keeps the
// hidden-dir filter so user-typed paths do not get dragged into
// .git, .cache, and similar metadata trees.
func TestFindPtStalkRoot_IncludeHiddenFindsRootUnderDotDir(t *testing.T) {
	tmp := t.TempDir()
	hidden := filepath.Join(tmp, ".cache", "pt", "host")
	makePtStalkSnapshot(t, hidden)

	// Default options skip the hidden subtree (existing behaviour).
	if _, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{}); !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("default opts: want ErrNotAPtStalkDir, got %v", err)
	}

	// IncludeHidden=true descends into the hidden subtree and finds
	// the root.
	got, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{IncludeHidden: true})
	if err != nil {
		t.Fatalf("IncludeHidden=true: %v", err)
	}
	want := resolvedAbs(t, hidden)
	if got != want {
		t.Fatalf("root mismatch:\n got=%s\nwant=%s", got, want)
	}
}

// TestFindPtStalkRoot_UnlimitedDepthFindsDeepRoot guards the Codex
// round-5 regression: archive inputs that previously walked the full
// extracted tree must still find a pt-stalk root nested deeper than
// DefaultMaxRootSearchDepth (8 levels). Passing
// MaxDepth=UnlimitedRootSearchDepth removes the depth cap.
func TestFindPtStalkRoot_UnlimitedDepthFindsDeepRoot(t *testing.T) {
	tmp := t.TempDir()
	// 12 directories below tmp; default cap (8) misses this.
	parts := []string{tmp, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	deep := filepath.Join(parts...)
	makePtStalkSnapshot(t, deep)

	// Default cap rejects.
	if _, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{}); !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("default opts: want ErrNotAPtStalkDir, got %v", err)
	}

	// Unlimited depth finds it.
	got, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{MaxDepth: UnlimitedRootSearchDepth})
	if err != nil {
		t.Fatalf("MaxDepth=UnlimitedRootSearchDepth: %v", err)
	}
	want := resolvedAbs(t, deep)
	if got != want {
		t.Fatalf("root mismatch:\n got=%s\nwant=%s", got, want)
	}
}

// TestFindPtStalkRoot_RootIsSymlinkedDir guards the Codex round-4
// regression: when rootDir is itself a symlink to a real pt-stalk
// directory, filepath.WalkDir uses Lstat on the root and reports it
// as a non-directory entry, so the callback's early return on
// non-directories would silently skip recognition. The fix resolves
// the root via filepath.EvalSymlinks before walking, restoring
// pre-feature behaviour where parse.Discover (which uses os.Stat)
// transparently followed root symlinks.
func TestFindPtStalkRoot_RootIsSymlinkedDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin or developer mode on Windows")
	}
	// Real pt-stalk capture lives outside the input tree.
	real := t.TempDir()
	makePtStalkSnapshot(t, real)

	// Input the operator passes is a symlink to it.
	parent := t.TempDir()
	link := filepath.Join(parent, "capture-symlink")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	got, err := FindPtStalkRoot(context.Background(), link, FindPtStalkRootOptions{})
	if err != nil {
		t.Fatalf("symlinked root rejected: %v", err)
	}
	want := resolvedAbs(t, real)
	if got != want {
		t.Fatalf("root mismatch:\n got=%s\nwant=%s", got, want)
	}
}

// TestFindPtStalkRoot_RootPlusNestedRootSurfacesAmbiguity guards the
// Codex round-3 regression: a tree where the input root itself is a
// pt-stalk root AND another distinct pt-stalk root exists below it
// must surface as a multi-root error rather than silently parsing
// only the top-level root. The previous unconditional top-level
// fast-path swallowed this ambiguity.
func TestFindPtStalkRoot_RootPlusNestedRootSurfacesAmbiguity(t *testing.T) {
	tmp := t.TempDir()
	// rootDir itself is a pt-stalk root (loose snapshot file at top).
	makePtStalkSnapshot(t, tmp)
	// And a second, distinct pt-stalk root lives in a nested subdir.
	nested := filepath.Join(tmp, "nested", "host")
	makePtStalkSnapshot(t, nested)

	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{})
	var mpe *MultiplePtStalkRootsError
	if !errors.As(err, &mpe) {
		t.Fatalf("want *MultiplePtStalkRootsError, got %T: %v", err, err)
	}
	if len(mpe.Roots) != 2 {
		t.Fatalf("want 2 roots, got %d: %v", len(mpe.Roots), mpe.Roots)
	}
	wantTop := resolvedAbs(t, tmp)
	wantNested := resolvedAbs(t, nested)
	// sort.StringsAreSorted already verified by the existing
	// TestFindPtStalkRoot_NestedMultiple test; here we check
	// membership.
	gotSet := map[string]bool{mpe.Roots[0]: true, mpe.Roots[1]: true}
	if !gotSet[wantTop] || !gotSet[wantNested] {
		t.Fatalf("missing root in:\n%v\nwant %s and %s", mpe.Roots, wantTop, wantNested)
	}
}

// Compile-time assertion that the WalkDirFunc signature is honored.
var _ fs.WalkDirFunc = func(path string, d fs.DirEntry, err error) error { return nil }

// TestFindPtStalkRoot_EntryCapCountsFiles guards against a regression
// where the entry counter only fired on directory entries. Trees with
// few directories but many files would silently bypass MaxEntries
// otherwise (PR #64 codex/copilot finding round 1).
func TestFindPtStalkRoot_EntryCapCountsFiles(t *testing.T) {
	tmp := t.TempDir()
	flat := filepath.Join(tmp, "flat")
	if err := os.MkdirAll(flat, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Drop 50 unrelated files into a single directory. With
	// MaxEntries = 5 the walker must short-circuit before it can
	// observe all of them.
	for i := 0; i < 50; i++ {
		name := filepath.Join(flat, "file_"+string(rune('a'+i%26))+"_"+itoa(i))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// Tight cap proves the counter fires for files; the walker
	// returns ErrNotAPtStalkDir because the synthetic directory
	// contains no pt-stalk-shaped file - the assertion that matters
	// is that the call returns *promptly* and does not visit every
	// entry.
	_, err := FindPtStalkRoot(context.Background(), tmp, FindPtStalkRootOptions{MaxEntries: 5})
	if !errors.Is(err, ErrNotAPtStalkDir) {
		t.Fatalf("want ErrNotAPtStalkDir under tight entry cap, got %v", err)
	}
}

// itoa formats a non-negative int into base-10 digits without
// pulling in strconv (already imported elsewhere in the package, but
// kept local for test isolation against future refactors).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestMultiplePtStalkRootsError_ErrorSortsExternalConstruction
// verifies that Error() defensively sorts an externally-constructed
// Roots slice so the doc-comment claim of byte-deterministic output
// holds even when the type is built outside FindPtStalkRoot
// (PR #64 copilot finding round 1).
func TestMultiplePtStalkRootsError_ErrorSortsExternalConstruction(t *testing.T) {
	mpe := &MultiplePtStalkRootsError{Roots: []string{"/z/host", "/a/host", "/m/host"}}
	msg := mpe.Error()
	if !strings.Contains(msg, "/a/host") || !strings.Contains(msg, "/z/host") {
		t.Fatalf("missing roots: %s", msg)
	}
	idxA := strings.Index(msg, "/a/host")
	idxM := strings.Index(msg, "/m/host")
	idxZ := strings.Index(msg, "/z/host")
	if !(idxA < idxM && idxM < idxZ) {
		t.Fatalf("Error() did not produce lexical order:\n%s", msg)
	}
	// e.Roots itself must not be mutated.
	if mpe.Roots[0] != "/z/host" {
		t.Fatalf("Error() mutated caller's Roots slice: %v", mpe.Roots)
	}
}
