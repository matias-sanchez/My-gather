package parse

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// Package-level regexes used by parseInnodbStatus. Lifted to avoid
// re-compiling on every call (see parse/meminfo.go for the same
// pattern).
var (
	reInnodbPendingReads     = regexp.MustCompile(`^Pending reads\s+(\d+)`)
	reInnodbPendingWritesSum = regexp.MustCompile(`LRU (\d+), flush list (\d+), single page (\d+)`)
	reInnodbHashRate         = regexp.MustCompile(`([\d.]+)\s+hash searches/s,\s+([\d.]+)\s+non-hash searches/s`)
	reInnodbHashTblSize      = regexp.MustCompile(`^Hash table size\s+(\d+)`)
	reInnodbHLL              = regexp.MustCompile(`^History list length (\d+)`)
	// SEMAPHORES entry: "--Thread 140148200982272 has waited at
	// ibuf0ibuf.cc line 3922 for 0 seconds the semaphore:" — capture
	// <file> and <line>.
	reInnodbThreadWaited = regexp.MustCompile(`^--Thread \d+ has waited at (\S+) line (\d+) `)
	// Partner line following the thread line: "Mutex at 0x..., Mutex
	// TRX_SYS created trx0sys.cc:599, locked by ...". We capture the
	// mutex name (TRX_SYS) so the breakdown has a readable label next
	// to the file:line key.
	reInnodbMutexName = regexp.MustCompile(`^Mutex at 0x[0-9a-fA-F]+, Mutex ([^\s]+) created`)
)

// parseInnodbStatus reads the text of `SHOW ENGINE INNODB STATUS` as
// captured by pt-stalk's -innodbstatus1 collector and extracts the
// four scalar views required by spec FR-014:
//
//   - SemaphoreCount: number of threads currently waiting on a
//     semaphore (counted from the SEMAPHORES section's "--Thread N
//     has waited …" lines).
//   - PendingReads / PendingWrites: values from the BUFFER POOL AND
//     MEMORY section's "Pending reads" / "Pending writes" lines.
//   - AHIActivity: hash table size + searches-per-second from the
//     INSERT BUFFER AND ADAPTIVE HASH INDEX section.
//   - HistoryListLength: TRANSACTIONS section "History list length".
//
// This is a targeted extractor rather than a full InnoDB status
// parser; we only read what the report renders.
func parseInnodbStatus(r io.Reader, sourcePath string) (*model.InnodbStatusData, []model.Diagnostic) {
	scanner := newLineScanner(r)

	var diagnostics []model.Diagnostic
	data := &model.InnodbStatusData{}

	var inSection string
	anySectionSeen := false
	threadWaited := 0

	// Aggregation key for SemaphoreSites so we can group by
	// (file, line, mutex) without another pass.
	type siteKey struct {
		file  string
		line  int
		mutex string
	}
	siteCounts := map[siteKey]int{}
	// pendingKey points at the most recently emitted orphan entry
	// (file, line, mutex="") so that if its paired `Mutex at ...` partner
	// lands on a subsequent line we can "upgrade" the entry by moving
	// the count under the (file, line, mutex=NAME) key. Older Percona /
	// MySQL captures sometimes omit the partner line entirely; we emit
	// on the thread line first and only upgrade when the partner
	// actually arrives, so orphans still appear in the breakdown.
	var pendingKey siteKey
	havePending := false

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		trimmed := strings.TrimSpace(line)

		// Section markers are all-dash lines whose NEXT line names the section.
		// Walk by just looking for the known labels directly.
		switch trimmed {
		case "SEMAPHORES", "BACKGROUND THREAD", "FILE I/O",
			"INSERT BUFFER AND ADAPTIVE HASH INDEX", "LOG",
			"BUFFER POOL AND MEMORY", "ROW OPERATIONS",
			"TRANSACTIONS":
			// Section state does not leak across sections.
			havePending = false
			inSection = trimmed
			anySectionSeen = true
			continue
		}

		switch inSection {
		case "SEMAPHORES":
			if m := reInnodbThreadWaited.FindStringSubmatch(trimmed); m != nil {
				threadWaited++
				lineNo, _ := strconv.Atoi(m[2])
				// Emit an orphan site immediately keyed on (file, line,
				// mutex=""). If the paired `Mutex at ...` partner lands
				// on a subsequent line we upgrade the entry below by
				// moving its count under the (file, line, mutex=NAME)
				// key. This guarantees every waited-at thread shows up
				// in the breakdown even when the partner line is absent.
				pendingKey = siteKey{file: m[1], line: lineNo, mutex: ""}
				siteCounts[pendingKey]++
				havePending = true
				continue
			}
			if havePending {
				if m := reInnodbMutexName.FindStringSubmatch(trimmed); m != nil {
					// Upgrade: move the count we stashed under the
					// empty-mutex key to the named-mutex key so the
					// two halves of the pair aggregate as one row.
					if siteCounts[pendingKey] > 0 {
						siteCounts[pendingKey]--
						if siteCounts[pendingKey] == 0 {
							delete(siteCounts, pendingKey)
						}
					}
					siteCounts[siteKey{file: pendingKey.file, line: pendingKey.line, mutex: m[1]}]++
					havePending = false
					continue
				}
				if trimmed == "" {
					continue
				}
				// Non-canonical: thread without its paired mutex. Leave
				// the orphan entry in place so it still renders.
				havePending = false
			}
		case "TRANSACTIONS":
			if m := reInnodbHLL.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.HistoryListLength = v
			}
		case "BUFFER POOL AND MEMORY":
			if m := reInnodbPendingReads.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.PendingReads = v
			}
			if strings.HasPrefix(trimmed, "Pending writes:") {
				if m := reInnodbPendingWritesSum.FindStringSubmatch(trimmed); m != nil {
					lru, _ := strconv.Atoi(m[1])
					fl, _ := strconv.Atoi(m[2])
					sp, _ := strconv.Atoi(m[3])
					data.PendingWrites = lru + fl + sp
				}
			}
		case "INSERT BUFFER AND ADAPTIVE HASH INDEX":
			// The first Hash table size line gives the configured size;
			// overwrite on each so we end up with the last-seen value.
			if m := reInnodbHashTblSize.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.AHIActivity.HashTableSize = v
			}
			if m := reInnodbHashRate.FindStringSubmatch(trimmed); m != nil {
				h, _ := strconv.ParseFloat(m[1], 64)
				nh, _ := strconv.ParseFloat(m[2], 64)
				data.AHIActivity.SearchesPerSec = h
				data.AHIActivity.NonHashSearchesPerSec = nh
			}
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("innodbstatus read: %v", err),
		})
	}

	// If the scan completed without encountering any recognised section
	// header, the file is almost certainly malformed or truncated.
	// Surface that so the reader doesn't silently see an empty InnoDB
	// status card.
	if !anySectionSeen {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityWarning,
			Message:    "innodbstatus: no recognised section headers; file may be malformed",
		})
	}

	data.SemaphoreCount = threadWaited

	// Materialise sites sorted desc by count, stable tie-break by
	// file ascending then line ascending so identical captures
	// render identically across runs.
	siteSum := 0
	if len(siteCounts) > 0 {
		sites := make([]model.SemaphoreSite, 0, len(siteCounts))
		for k, c := range siteCounts {
			sites = append(sites, model.SemaphoreSite{
				File:      k.file,
				Line:      k.line,
				MutexName: k.mutex,
				WaitCount: c,
			})
			siteSum += c
		}
		model.SortSemaphoreSites(sites)
		data.SemaphoreSites = sites
	}

	// Invariant check: threadWaited counts thread lines; siteSum counts
	// entries emitted into siteCounts (orphans under mutex="" plus
	// paired entries under mutex=NAME). The parser now emits on every
	// thread line, so these should always agree; a mismatch would
	// indicate a bookkeeping bug — surface it so it can't go silent.
	if siteSum != threadWaited {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityWarning,
			Message: fmt.Sprintf(
				"innodbstatus: SemaphoreSites wait-count sum (%d) does not equal threads-waited count (%d); contention breakdown may be missing rows",
				siteSum, threadWaited),
		})
	}

	return data, diagnostics
}
