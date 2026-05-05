package parse

import (
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// Per-sample "seen" bits. A complete /proc/meminfo dump always
// contains every mCore… key, so a sample missing any of them is a
// truncation candidate. We require "seen" rather than "non-zero"
// because zero is a valid reading (e.g. SwapTotal on swapless hosts).
const (
	mCoreMemTotal uint = 1 << iota
	mCoreMemFree
	mCoreMemAvailable
	mCoreBuffers
	mCoreCached
	mCoreSwapTotal
	mCoreSwapFree

	mCoreAll = (1 << 7) - 1
)

// ParseEnvMeminfo extracts the scalar fields used by the Environment
// panel from a raw -meminfo file contents string.
//
// The file is the concatenation of one or more /proc/meminfo dumps
// separated by `TS <epoch> …` boundary lines. Each sample is scanned
// INDEPENDENTLY; keys from different samples are never mixed.
//
// Selection (newest-first):
//  1. Prefer a sample where every core /proc/meminfo field (MemTotal,
//     MemFree, MemAvailable, Buffers, Cached, SwapTotal, SwapFree)
//     was observed, regardless of its value. This guards against a
//     last sample truncated shortly after MemTotal — otherwise
//     missing-past-truncation fields would read as 0 and the render
//     would confidently misreport e.g. a machine as swapless.
//  2. Otherwise, fall back to the newest sample with MemTotal set.
//  3. Otherwise, return the newest sample with any tracked key, or
//     nil when the input contains nothing usable.
//
// Never returns an error: malformed or empty input yields nil, which
// the template renders as "—".
func ParseEnvMeminfo(content string) *model.EnvMeminfo {
	data, _ := ParseEnvMeminfoWithDiagnostics(content, "")
	return data
}

// ParseEnvMeminfoWithDiagnostics extracts environment meminfo fields
// and reports every compatibility fallback used while selecting a
// sample. The returned EnvMeminfo matches ParseEnvMeminfo exactly.
func ParseEnvMeminfoWithDiagnostics(content string, sourcePath string) (*model.EnvMeminfo, []model.Diagnostic) {
	type sample struct {
		data *model.EnvMeminfo
		seen uint
		line int
	}
	var (
		samples     []sample
		cur         *sample
		diagnostics []model.Diagnostic
		lineNum     int
	)
	addDiag := func(line int, msg string) {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Location:   "line " + strconv.Itoa(line),
			Severity:   model.SeverityWarning,
			Message:    msg,
		})
	}
	flush := func() {
		if cur != nil && cur.seen != 0 {
			samples = append(samples, *cur)
		}
		cur = nil
	}
	setInt := func(dst *int64, bit uint, key string, rest string) {
		// "MemTotal:       32654396 kB" — after the colon, fields are
		// [number, unit]. HugePages_Total has no unit so we just take
		// the first numeric field.
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return
		}
		v, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			addDiag(lineNum, "env meminfo: non-numeric value for "+key+": "+fields[0])
			return
		}
		*dst = v
		cur.seen |= bit
	}
	// Auxiliary (non-core) keys still count as "seen" for the
	// any-tracked-key fallback, but don't participate in mCoreAll.
	const mAux uint = 1 << 7
	for _, line := range strings.Split(content, "\n") {
		lineNum++
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "TS ") {
			flush()
			cur = &sample{data: &model.EnvMeminfo{}, line: lineNum}
			continue
		}
		if cur == nil {
			// Input without any TS boundaries — treat the whole file
			// as a single sample.
			cur = &sample{data: &model.EnvMeminfo{}, line: lineNum}
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		rest := line[idx+1:]
		switch key {
		case "MemTotal":
			setInt(&cur.data.MemTotalKB, mCoreMemTotal, key, rest)
		case "MemFree":
			setInt(&cur.data.MemFreeKB, mCoreMemFree, key, rest)
		case "MemAvailable":
			setInt(&cur.data.MemAvailableKB, mCoreMemAvailable, key, rest)
		case "Buffers":
			setInt(&cur.data.BuffersKB, mCoreBuffers, key, rest)
		case "Cached":
			setInt(&cur.data.CachedKB, mCoreCached, key, rest)
		case "SwapTotal":
			setInt(&cur.data.SwapTotalKB, mCoreSwapTotal, key, rest)
		case "SwapFree":
			setInt(&cur.data.SwapFreeKB, mCoreSwapFree, key, rest)
		case "HugePages_Total":
			setInt(&cur.data.HugePagesTotal, mAux, key, rest)
		case "AnonHugePages":
			setInt(&cur.data.AnonHugePagesKB, mAux, key, rest)
		}
	}
	flush()

	// 1. Newest sample with every core field observed.
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].seen&mCoreAll == mCoreAll {
			if i != len(samples)-1 {
				last := samples[len(samples)-1]
				addDiag(last.line, "env meminfo: newest sample is incomplete ("+missingEnvMeminfoCoreFields(last.seen)+"); using older complete sample")
			}
			return samples[i].data, diagnostics
		}
	}
	// 2. Newest sample that at least reached MemTotal.
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].seen&mCoreMemTotal != 0 {
			addDiag(samples[i].line, "env meminfo: no complete sample found; using newest sample with MemTotal despite missing core fields: "+missingEnvMeminfoCoreFields(samples[i].seen))
			return samples[i].data, diagnostics
		}
	}
	// 3. Give up gracefully — nothing usable.
	if len(samples) == 0 {
		return nil, diagnostics
	}
	last := samples[len(samples)-1]
	addDiag(last.line, "env meminfo: no sample with MemTotal found; using newest partial sample")
	return last.data, diagnostics
}

func missingEnvMeminfoCoreFields(seen uint) string {
	missing := make([]string, 0, 7)
	if seen&mCoreMemTotal == 0 {
		missing = append(missing, "MemTotal")
	}
	if seen&mCoreMemFree == 0 {
		missing = append(missing, "MemFree")
	}
	if seen&mCoreMemAvailable == 0 {
		missing = append(missing, "MemAvailable")
	}
	if seen&mCoreBuffers == 0 {
		missing = append(missing, "Buffers")
	}
	if seen&mCoreCached == 0 {
		missing = append(missing, "Cached")
	}
	if seen&mCoreSwapTotal == 0 {
		missing = append(missing, "SwapTotal")
	}
	if seen&mCoreSwapFree == 0 {
		missing = append(missing, "SwapFree")
	}
	return strings.Join(missing, ", ")
}
