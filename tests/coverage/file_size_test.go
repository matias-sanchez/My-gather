package coverage_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

const governedSourceLineLimit = 1000

var governedSourceExtensions = map[string]bool{
	".css":  true,
	".go":   true,
	".js":   true,
	".jsx":  true,
	".sh":   true,
	".tmpl": true,
	".ts":   true,
	".tsx":  true,
}

// TestGovernedSourceFileLineLimit enforces Constitution Principle XV:
// maintained first-party source files must stay below the god-file line limit.
func TestGovernedSourceFileLineLimit(t *testing.T) {
	root := goldens.RepoRoot(t)
	paths := gitTrackedFiles(t, root)

	var offenders []string
	for _, path := range paths {
		if !isGovernedSourcePath(path) {
			continue
		}
		lines := countLines(t, filepath.Join(root, filepath.FromSlash(path)))
		if lines > governedSourceLineLimit {
			offenders = append(offenders, path+" has "+itoa(lines)+" lines")
		}
	}

	if len(offenders) == 0 {
		return
	}
	sort.Strings(offenders)
	t.Fatalf(
		"governed source files must not exceed %d lines:\n%s",
		governedSourceLineLimit,
		strings.Join(offenders, "\n"),
	)
}

func gitTrackedFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func isGovernedSourcePath(path string) bool {
	if strings.HasPrefix(path, "_references/") || strings.HasPrefix(path, "testdata/") {
		return false
	}
	if path == "render/assets/chart.min.js" || path == "render/assets/chart.min.css" {
		return false
	}
	return governedSourceExtensions[filepath.Ext(path)]
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		lines++
	}
	return lines
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
