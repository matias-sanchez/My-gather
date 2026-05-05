package coverage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

func TestSpecKitGitFeatureUsesCanonicalHelperPaths(t *testing.T) {
	root := goldens.RepoRoot(t)

	retiredBashHelper := filepath.Join(root, ".specify", "extensions", "git", "scripts", "bash", "git-common.sh")
	if _, err := os.Stat(retiredBashHelper); err == nil {
		t.Fatalf("retired Bash helper path still exists: %s", retiredBashHelper)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat retired Bash helper path: %v", err)
	}

	bash := readRepoFile(t, root, ".specify/extensions/git/scripts/bash/create-new-feature.sh")
	if !strings.Contains(bash, ".specify/scripts/bash/common.sh") {
		t.Fatalf("Bash git feature script must source the canonical installed common.sh")
	}
	for _, forbidden := range []string{
		`$_PROJECT_ROOT/scripts/bash/common.sh`,
		`$SCRIPT_DIR/git-common.sh`,
		"git-common.sh next to this script",
		"minimal fallback",
		"source checkout fallback",
	} {
		if strings.Contains(bash, forbidden) {
			t.Fatalf("Bash git feature script still contains retired helper path %q", forbidden)
		}
	}

	powershell := readRepoFile(t, root, ".specify/extensions/git/scripts/powershell/create-new-feature.ps1")
	if !strings.Contains(powershell, "Test-HasGit -RepoRoot $repoRoot") {
		t.Fatalf("PowerShell git feature script must use the canonical git helper")
	}
	for _, forbidden := range []string{
		".specify/scripts/powershell/common.ps1",
		"scripts/powershell/common.ps1",
		"minimal fallback",
		"source checkout fallback",
	} {
		if strings.Contains(powershell, forbidden) {
			t.Fatalf("PowerShell git feature script still contains retired helper path %q", forbidden)
		}
	}
}
