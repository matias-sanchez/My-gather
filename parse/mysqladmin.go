package parse

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// parseMysqladmin implements the pt-mext delta-computation algorithm
// natively in Go (see research R8, spec FR-028). Line-for-line
// correspondence with `_references/pt-mext/pt-mext-improved.cpp`:
//
//  1. Only lines starting with `|` are data rows (pt-mext's
//     `skip_line`).
//  2. A row whose first column is literally `Variable_name` marks a
//     new-sample boundary (pt-mext line 42–45).
//  3. For counter variables: Deltas[v][col] = current - previous. The
//     first column's "delta" is the raw initial tally and is excluded
//     from aggregates (pt-mext lines 47–54).
//  4. Aggregates (total, min, max, avg) are over columns 2..N using
//     col==2 to seed min (pt-mext line 51).
//
// Improvements over the C++ reference (research R8 A/B/C):
//   - Counter/gauge classification via isCounter().
//   - float64 aggregates (avg doesn't truncate to zero).
//   - Variable drift: NaN in missing slots + SeverityWarning per
//     variable.
//
// Snapshot-boundary reset (FR-030 / R8 D) is handled by the caller —
// per-collector Discover integration resets the parser between
// concatenated -mysqladmin files and inserts NaN at each boundary via
// ResetPrevious + the returned SnapshotBoundaries slice.
func parseMysqladmin(r io.Reader, snapshotStart time.Time, sourcePath string) (*model.MysqladminData, []model.Diagnostic) {
	p := newMysqladminParser(sourcePath, snapshotStart)
	p.parseOneFile(r)
	return p.finish()
}

// mysqladminParser holds the incremental state needed to parse one or
// more concatenated pt-stalk -mysqladmin files into a single
// MysqladminData. Callers use Reset to introduce a snapshot boundary.
type mysqladminParser struct {
	sourcePath    string
	timestampBase time.Time

	// col is the 1-based sample index (matching pt-mext's `col`).
	col int
	// previous holds the last raw value seen per counter, across all
	// samples processed since Reset/start.
	previous map[string]int64
	// values[name][i] = raw value at sample index i (0-based in Go).
	values map[string][]float64
	// isCounter memoises classification per variable.
	isCounter map[string]bool
	// sampleTimestamps is one time per column, in column order.
	sampleTimestamps []time.Time
	// boundaries lists the 0-based column indexes at which a new file
	// begins (always includes 0).
	boundaries []int
	// nameOrder preserves first-seen order for stable iteration.
	nameOrder []string
	nameSeen  map[string]struct{}

	diagnostics []model.Diagnostic
}

func newMysqladminParser(sourcePath string, t time.Time) *mysqladminParser {
	return &mysqladminParser{
		sourcePath:    sourcePath,
		timestampBase: t,
		previous:      map[string]int64{},
		values:        map[string][]float64{},
		isCounter:     map[string]bool{},
		nameSeen:      map[string]struct{}{},
		boundaries:    []int{0},
	}
}

// ResetForNewSnapshot records a snapshot boundary at the next sample
// column and resets the `previous` map so deltas don't span across
// snapshots (spec FR-030).
//
// Safe to call before any samples have been added — the default
// boundaries slice already has 0 as its first entry.
func (p *mysqladminParser) ResetForNewSnapshot(newSnapshotStart time.Time) {
	p.previous = map[string]int64{}
	p.timestampBase = newSnapshotStart
	if p.col > 0 {
		// The NEXT column we ingest will be a boundary.
		p.boundaries = append(p.boundaries, p.col)
		p.diagnostics = append(p.diagnostics, model.Diagnostic{
			SourceFile: p.sourcePath,
			Severity:   model.SeverityInfo,
			Message:    fmt.Sprintf("mysqladmin: snapshot boundary at column %d (FR-030)", p.col),
		})
	}
}

// parseOneFile consumes one file's worth of pipe-delimited rows.
func (p *mysqladminParser) parseOneFile(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	for scanner.Scan() {
		raw := scanner.Text()
		if len(raw) == 0 || raw[0] != '|' {
			continue
		}
		name, valueStr, ok := splitTwoPipeFields(raw)
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		valueStr = strings.TrimSpace(valueStr)

		if name == "Variable_name" {
			// New sample column.
			p.col++
			continue
		}
		if p.col == 0 {
			// Data row before any header; ignore.
			continue
		}

		// Register the name on first sight.
		if _, seen := p.nameSeen[name]; !seen {
			p.nameSeen[name] = struct{}{}
			p.nameOrder = append(p.nameOrder, name)
			p.isCounter[name] = classifyAsCounter(name)
		}

		// Parse as int64 (pt-mext uses long long). Non-numeric values
		// parse to 0, matching pt-mext's strtol behaviour.
		v, _ := strconv.ParseInt(valueStr, 10, 64)

		// Ensure the per-variable slice is padded out to (col-1). Any
		// missing prior slots represent drift; fill with NaN and emit
		// exactly one warning per variable.
		slots := p.values[name]
		targetLen := p.col // 0-based index for col-th sample is col-1, but we want slots[col-1] to be the value
		for len(slots) < targetLen-1 {
			slots = append(slots, math.NaN())
		}

		// Compute the per-sample output according to counter/gauge.
		var out float64
		if p.isCounter[name] {
			prev := p.previous[name]
			if p.col == 1 || p.atBoundaryFor(p.col-1) {
				// Initial tally (col 1) OR first sample after a snapshot
				// boundary: pt-mext-compatible — store raw for col 1;
				// NaN for post-boundary first slots per FR-030.
				if p.col == 1 {
					out = float64(v)
				} else {
					out = math.NaN()
				}
			} else {
				out = float64(v - prev)
			}
			p.previous[name] = v
		} else {
			// Gauge: raw per-sample value.
			out = float64(v)
		}

		if len(slots) == targetLen-1 {
			slots = append(slots, out)
		} else if len(slots) == targetLen {
			// This shouldn't normally happen; pt-mext would overwrite.
			slots[targetLen-1] = out
		}
		p.values[name] = slots
	}
	// Capture the first sample's timestamp for this column block.
	// (We synthesise 1-second spacing — mysqladmin samples are 1s apart.)
	// We record timestamps lazily in finish().
}

// atBoundaryFor returns true if the 0-based sample index is the first
// column of a non-initial snapshot block (i.e., it's in boundaries and
// isn't 0).
func (p *mysqladminParser) atBoundaryFor(colZeroBased int) bool {
	for _, b := range p.boundaries {
		if b == colZeroBased && colZeroBased != 0 {
			return true
		}
	}
	return false
}

// finish pads any trailing drift slots to the total column count and
// emits the final MysqladminData.
func (p *mysqladminParser) finish() (*model.MysqladminData, []model.Diagnostic) {
	if p.col == 0 || len(p.nameOrder) == 0 {
		return nil, p.diagnostics
	}
	totalCols := p.col

	// Emit a single drift warning per variable that ended short of
	// totalCols, then pad with NaN.
	for _, name := range p.nameOrder {
		vs := p.values[name]
		if len(vs) < totalCols {
			p.diagnostics = append(p.diagnostics, model.Diagnostic{
				SourceFile: p.sourcePath,
				Severity:   model.SeverityWarning,
				Message:    fmt.Sprintf("mysqladmin: variable %q missing from %d trailing sample(s)", name, totalCols-len(vs)),
			})
			for len(vs) < totalCols {
				vs = append(vs, math.NaN())
			}
			p.values[name] = vs
		}
	}

	// Synthesise per-column timestamps. Simplest: 1-second spacing from
	// the timestampBase. Boundaries keep using the same base until
	// ResetForNewSnapshot was called.
	if len(p.sampleTimestamps) != totalCols {
		// For the MVP we synthesise a single 1-second-spaced ladder
		// from the last-set timestampBase. When multi-file mode is used
		// the caller is expected to invoke SetTimestampsForNextBlock.
		p.sampleTimestamps = make([]time.Time, totalCols)
		for i := 0; i < totalCols; i++ {
			p.sampleTimestamps[i] = p.timestampBase.Add(time.Duration(i) * time.Second)
		}
	}

	// Produce sorted variable names for deterministic output.
	varNames := make([]string, len(p.nameOrder))
	copy(varNames, p.nameOrder)
	sort.Strings(varNames)

	out := &model.MysqladminData{
		VariableNames:      varNames,
		SampleCount:        totalCols,
		Timestamps:         p.sampleTimestamps,
		Deltas:             p.values,
		IsCounter:          p.isCounter,
		SnapshotBoundaries: p.boundaries,
	}
	return out, p.diagnostics
}

// splitTwoPipeFields extracts the Variable_name and Value fields from
// a single line of the form "| name | value |". Returns ok == false
// when the line does not contain at least two `|` separators after the
// leading one, matching pt-mext's implicit expectations.
func splitTwoPipeFields(line string) (string, string, bool) {
	// pt-mext: name = line[1 : find("|", 1)],
	// value = line[find("|",1)+1 : find("|", that)-1]
	i1 := strings.Index(line[1:], "|")
	if i1 < 0 {
		return "", "", false
	}
	i1 += 1 // index in full line
	name := line[1:i1]
	i2 := strings.Index(line[i1+1:], "|")
	if i2 < 0 {
		// No closing pipe — matches pt-mext behaviour of returning
		// malformed garbage; we treat as unparseable.
		return "", "", false
	}
	i2 += i1 + 1
	value := line[i1+1 : i2]
	return name, value, true
}

// classifyAsCounter decides whether `name` is a monotonically
// increasing counter (true) or a point-in-time gauge (false).
//
// The decision follows the same logic Percona's mysqld_exporter uses
// to set prometheus.CounterValue vs GaugeValue on SHOW GLOBAL STATUS
// scrapes (see mysqld_exporter/collector/global_status.go:37 for the
// regex and lines 134-172 for the Innodb_buffer_pool_pages
// sub-switch). Two layers:
//
//  1. Explicit JSON overrides from parse/mysql-status-types.json —
//     used for variables that the exporter reports as Untyped (i.e.
//     unmatched by its regex). The JSON encodes the MySQL reference
//     manual's counter/gauge semantics for the variables we routinely
//     see in pt-stalk captures.
//  2. Ported regex + sub-switch — matches the exporter bit-for-bit.
//  3. Default: gauge. Safer than a bogus delta if we don't know.
func classifyAsCounter(name string) bool {
	types := loadStatusTypes()
	if types != nil {
		if _, ok := types.counters[name]; ok {
			return true
		}
		if _, ok := types.gauges[name]; ok {
			return false
		}
	}
	m := exporterStatusRE.FindStringSubmatch(strings.ToLower(name))
	if m == nil {
		return false
	}
	switch m[1] {
	case "com", "handler", "connection_errors", "innodb_rows", "performance_schema":
		return true
	case "innodb_buffer_pool_pages":
		switch m[2] {
		case "data", "free", "misc", "old", "dirty", "total":
			return false
		default:
			return true
		}
	}
	return false
}

// exporterStatusRE mirrors mysqld_exporter's globalStatusRE (same
// file, same line-number). Grouping:
//
//	m[1] -> family (com / handler / connection_errors /
//	        innodb_buffer_pool_pages / innodb_rows / performance_schema)
//	m[2] -> suffix the family didn't strip
var exporterStatusRE = regexp.MustCompile(
	`^(com|handler|connection_errors|innodb_buffer_pool_pages|innodb_rows|performance_schema)_(.*)$`,
)

//go:embed mysql-status-types.json
var embeddedStatusTypesJSON []byte

type statusTypeTables struct {
	counters map[string]struct{}
	gauges   map[string]struct{}
}

var (
	statusTypes     *statusTypeTables
	statusTypesOnce sync.Once
)

func loadStatusTypes() *statusTypeTables {
	statusTypesOnce.Do(func() {
		var raw struct {
			ExplicitCounters []string `json:"explicit_counters"`
			ExplicitGauges   []string `json:"explicit_gauges"`
		}
		// The override file is embedded at build time (//go:embed).
		// A parse error therefore means the JSON was broken at
		// compile time — a programmer error, not a user-runtime
		// condition. Panic loudly so the misclassification can't
		// quietly ship; there is no safe fallback because silent
		// degradation flips ~170 counters back to gauges and
		// corrupts downstream advisor findings.
		if err := json.Unmarshal(embeddedStatusTypesJSON, &raw); err != nil {
			panic(fmt.Sprintf("parse/mysql-status-types.json: malformed embedded JSON: %v", err))
		}
		t := &statusTypeTables{
			counters: make(map[string]struct{}, len(raw.ExplicitCounters)),
			gauges:   make(map[string]struct{}, len(raw.ExplicitGauges)),
		}
		for _, n := range raw.ExplicitCounters {
			t.counters[n] = struct{}{}
		}
		for _, n := range raw.ExplicitGauges {
			t.gauges[n] = struct{}{}
		}
		statusTypes = t
	})
	return statusTypes
}
