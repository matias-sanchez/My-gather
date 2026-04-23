package render

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/chart.min.js
var embeddedChartJS string

//go:embed assets/chart.min.css
var embeddedChartCSS string

//go:embed assets/app.js
var embeddedAppJS string

//go:embed assets/app.css
var embeddedAppCSS string

//go:embed assets/mysql-defaults.json
var embeddedMySQLDefaultsJSON []byte

//go:embed assets/mysqladmin-categories.json
var embeddedMysqladminCategoriesJSON []byte

//go:embed assets/logo.png
var embeddedLogoPNG []byte

type mysqladminCategoryDef struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	Matchers        []string `json:"matchers"`
	ExcludeMatchers []string `json:"exclude_matchers"`
	Members         []string `json:"members"`
}

var (
	mysqladminCategories     []mysqladminCategoryDef
	mysqladminCategoriesOnce sync.Once
)

func loadMysqladminCategories() []mysqladminCategoryDef {
	mysqladminCategoriesOnce.Do(func() {
		var v struct {
			Categories []mysqladminCategoryDef `json:"categories"`
		}
		// Embedded at build time — a parse error is a programmer
		// error, not a user-runtime condition. Fail loudly instead
		// of silently producing an uncategorised chart view.
		if err := json.Unmarshal(embeddedMysqladminCategoriesJSON, &v); err != nil {
			panic(fmt.Sprintf("render/assets/mysqladmin-categories.json: malformed embedded JSON: %v", err))
		}
		mysqladminCategories = v.Categories
	})
	return mysqladminCategories
}

// classifyMysqladminCategory returns the slugs of every category that
// claims `name`. Matching is case-insensitive on prefixes. Explicit
// `members` take precedence; `exclude_matchers` carve out a subset of
// a broader matcher (e.g., buffer-pool excludes the read/write
// subviews so they live in InnoDB Reads / Writes instead).
func classifyMysqladminCategory(cats []mysqladminCategoryDef, name string) []string {
	lower := strings.ToLower(name)
	var hits []string
	for _, c := range cats {
		// Members list — highest priority.
		matched := false
		for _, m := range c.Members {
			if strings.EqualFold(m, name) {
				matched = true
				break
			}
		}
		if !matched {
			for _, mp := range c.Matchers {
				if strings.HasPrefix(lower, strings.ToLower(mp)) {
					matched = true
					break
				}
			}
			if matched {
				for _, ex := range c.ExcludeMatchers {
					if strings.HasPrefix(lower, strings.ToLower(ex)) {
						matched = false
						break
					}
				}
			}
		}
		if matched {
			hits = append(hits, c.Key)
		}
	}
	return hits
}

// mysqlDefaults is the parsed default-values map, populated on first use.
var (
	mysqlDefaults     map[string]string
	mysqlDefaultsOnce sync.Once
)

func loadMySQLDefaults() map[string]string {
	mysqlDefaultsOnce.Do(func() {
		var v struct {
			Defaults map[string]string `json:"defaults"`
		}
		// Embedded at build time — a parse error is a programmer
		// error. Fail loudly so the "non-default" badges can't
		// silently lose their reference map.
		if err := json.Unmarshal(embeddedMySQLDefaultsJSON, &v); err != nil {
			panic(fmt.Sprintf("render/assets/mysql-defaults.json: malformed embedded JSON: %v", err))
		}
		mysqlDefaults = v.Defaults
	})
	return mysqlDefaults
}

// classifyVariable compares a captured variable value against the
// documented compiled-in default. Returns:
//   - "default"  — value matches the documented default
//   - "modified" — value differs from the documented default
//   - "unknown"  — no default is documented for this variable
//
// Matching is tolerant: whitespace-trimmed, case-insensitive, and
// comma-separated values (e.g. sql_mode, tls_version) compared as
// sets so member order does not flag a default as modified.
func classifyVariable(defaults map[string]string, name, observed string) string {
	def, ok := defaults[name]
	if !ok {
		return "unknown"
	}
	if normalisedEqual(def, observed) {
		return "default"
	}
	if commaSetsEqual(def, observed) {
		return "default"
	}
	return "modified"
}

func normalisedEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func commaSetsEqual(a, b string) bool {
	if !strings.Contains(a, ",") && !strings.Contains(b, ",") {
		return false
	}
	split := func(s string) []string {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			out = append(out, strings.ToLower(p))
		}
		sort.Strings(out)
		return out
	}
	xs, ys := split(a), split(b)
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
}
