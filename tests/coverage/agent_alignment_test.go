package coverage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

func TestAgentContextFeaturePointersStayAligned(t *testing.T) {
	root := goldens.RepoRoot(t)

	agents := readRepoFile(t, root, "AGENTS.md")
	claude := readRepoFile(t, root, "CLAUDE.md")

	var pointer struct {
		FeatureDirectory string `json:"feature_directory"`
	}
	if err := json.Unmarshal(
		[]byte(readRepoFile(t, root, ".specify/feature.json")),
		&pointer,
	); err != nil {
		t.Fatalf("parse .specify/feature.json: %v", err)
	}
	if pointer.FeatureDirectory == "" {
		t.Fatalf(".specify/feature.json must name a feature directory")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(pointer.FeatureDirectory))); err != nil {
		t.Fatalf(".specify/feature.json points at missing feature directory %q: %v", pointer.FeatureDirectory, err)
	}

	featureName := filepath.Base(pointer.FeatureDirectory)
	for name, text := range map[string]string{"AGENTS.md": agents, "CLAUDE.md": claude} {
		block := speckitBlock(t, text)
		if strings.Contains(block, "No active feature.") {
			if !strings.Contains(block, "Active feature: **none**") {
				t.Fatalf("%s no-active block must mark active feature as none", name)
			}
			if !strings.Contains(strings.ToLower(block), "latest shipped feature is **"+featureName+"**") {
				t.Fatalf("%s no-active block must name %s as latest shipped", name, featureName)
			}
			if !strings.Contains(block, pointer.FeatureDirectory) {
				t.Fatalf("%s no-active block must reference %q", name, pointer.FeatureDirectory)
			}
			continue
		}
		wantMarker := "Active feature: **" + featureName + "**"
		if !strings.Contains(block, wantMarker) {
			t.Fatalf("%s active block missing %q", name, wantMarker)
		}
		for _, required := range []string{
			pointer.FeatureDirectory + "/plan.md",
			pointer.FeatureDirectory + "/spec.md",
			pointer.FeatureDirectory + "/tasks.md",
			".specify/memory/constitution.md",
		} {
			if !strings.Contains(block, required) {
				t.Fatalf("%s active block missing %q", name, required)
			}
		}
	}

	for name, text := range map[string]string{
		"AGENTS.md": agents,
		"CLAUDE.md": claude,
	} {
		if !strings.Contains(text, "When `AGENTS.md` and `CLAUDE.md` say there is no active feature") {
			t.Fatalf("%s must document the no-active-feature feature.json contract", name)
		}
	}
}

func TestAgentSkillTreesStayAligned(t *testing.T) {
	root := goldens.RepoRoot(t)
	codexSkills := skillSlugs(t, filepath.Join(root, ".agents", "skills"))
	claudeSkills := skillSlugs(t, filepath.Join(root, ".claude", "skills"))

	allowedClaudeOnly := map[string]bool{
		"pr-review-fix-my-gather":     true,
		"pr-review-loop-my-gather":    true,
		"pr-review-trigger-my-gather": true,
	}

	for slug := range codexSkills {
		if !claudeSkills[slug] {
			t.Fatalf("Codex skill %q is missing from .claude/skills", slug)
		}
	}
	for slug := range claudeSkills {
		if codexSkills[slug] || allowedClaudeOnly[slug] {
			continue
		}
		t.Fatalf("Claude skill %q is missing from .agents/skills", slug)
	}

	for slug := range codexSkills {
		codexSkill := readRepoFile(t, root, filepath.Join(".agents", "skills", slug, "SKILL.md"))
		claudeSkill := readRepoFile(t, root, filepath.Join(".claude", "skills", slug, "SKILL.md"))
		if normalizeAgentSkill(codexSkill) != normalizeAgentSkill(claudeSkill) {
			t.Fatalf("skill %q differs beyond approved agent-specific terms", slug)
		}
	}
}

func TestCodexReviewSkillsRemainStartupOnly(t *testing.T) {
	root := goldens.RepoRoot(t)
	agents := readRepoFile(t, root, "AGENTS.md")

	for _, slug := range []string{
		"pr-review-fix-my-gather",
		"pr-review-loop-my-gather",
		"pr-review-trigger-my-gather",
	} {
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", slug, "SKILL.md")); err == nil {
			t.Fatalf("%s must not be duplicated under .agents/skills; Codex loads it from ~/.codex/skills", slug)
		}
		if !strings.Contains(agents, "~/.codex/skills/"+slug+"/") {
			t.Fatalf("AGENTS.md must document Codex startup skill %s", slug)
		}
	}
}

func TestAgentContextsCarryEnglishArtifactConvention(t *testing.T) {
	root := goldens.RepoRoot(t)
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		text := readRepoFile(t, root, name)
		if !strings.Contains(text, "English-only for all checked-in artifacts") {
			t.Fatalf("%s must carry the English artifact convention", name)
		}
	}
}

func readRepoFile(t *testing.T, root string, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func speckitBlock(t *testing.T, text string) string {
	t.Helper()
	start := strings.Index(text, "<!-- SPECKIT START -->")
	end := strings.Index(text, "<!-- SPECKIT END -->")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("missing valid SPECKIT block")
	}
	return text[start : end+len("<!-- SPECKIT END -->")]
}

func skillSlugs(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read skills dir %s: %v", dir, err)
	}
	slugs := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Name(), "SKILL.md")); err == nil {
			slugs[entry.Name()] = true
		}
	}
	return slugs
}

func normalizeAgentSkill(text string) string {
	replacements := map[string]string{
		"AGENTS.md":   "{AGENT_CONTEXT}",
		"CLAUDE.md":   "{AGENT_CONTEXT}",
		".agents":     "{AGENT_DIR}",
		".claude":     "{AGENT_DIR}",
		"Codex only":  "AGENT only",
		"CLAUDE only": "AGENT only",
	}
	for old, replacement := range replacements {
		text = strings.ReplaceAll(text, old, replacement)
	}
	return text
}
