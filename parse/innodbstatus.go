package parse

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
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
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)

	var diagnostics []model.Diagnostic
	data := &model.InnodbStatusData{}

	var inSection string
	threadWaited := 0
	rePendingReads := regexp.MustCompile(`^Pending reads\s+(\d+)`)
	rePendingWritesSum := regexp.MustCompile(`LRU (\d+), flush list (\d+), single page (\d+)`)
	reHashRate := regexp.MustCompile(`([\d.]+)\s+hash searches/s,\s+([\d.]+)\s+non-hash searches/s`)
	reHashTblSize := regexp.MustCompile(`^Hash table size\s+(\d+)`)
	reHLL := regexp.MustCompile(`^History list length (\d+)`)
	// SEMAPHORES entry: "--Thread 140148200982272 has waited at
	// ibuf0ibuf.cc line 3922 for 0 seconds the semaphore:" — capture
	// <file> and <line>.
	reThreadWaited := regexp.MustCompile(`^--Thread \d+ has waited at (\S+) line (\d+) `)
	// Partner line following the thread line: "Mutex at 0x..., Mutex
	// TRX_SYS created trx0sys.cc:599, locked by ...". We capture the
	// mutex name (TRX_SYS) so the breakdown has a readable label next
	// to the file:line key.
	reMutexName := regexp.MustCompile(`^Mutex at 0x[0-9a-fA-F]+, Mutex ([^\s]+) created`)

	// Aggregation key for SemaphoreSites so we can group by
	// (file, line, mutex) without another pass.
	type siteKey struct {
		file  string
		line  int
		mutex string
	}
	siteCounts := map[siteKey]int{}
	// awaitingMutex holds the pending thread's file/line while we
	// look for its partner "Mutex ..." line on the next non-blank
	// row. Only one pending record at a time: pt-stalk always emits
	// the pair consecutively separated by at most whitespace lines.
	var pendingFile string
	var pendingLine int
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
			inSection = trimmed
			continue
		}

		switch inSection {
		case "SEMAPHORES":
			if m := reThreadWaited.FindStringSubmatch(trimmed); m != nil {
				threadWaited++
				// Flush a previous unmatched pending record with no
				// mutex name so consecutive thread lines (rare but
				// possible) don't lose their file:line.
				if havePending {
					siteCounts[siteKey{file: pendingFile, line: pendingLine, mutex: ""}]++
				}
				pendingFile = m[1]
				pendingLine, _ = strconv.Atoi(m[2])
				havePending = true
				continue
			}
			if havePending {
				if m := reMutexName.FindStringSubmatch(trimmed); m != nil {
					siteCounts[siteKey{file: pendingFile, line: pendingLine, mutex: m[1]}]++
					havePending = false
					continue
				}
				// If we hit a blank line or an unrelated line, the
				// mutex partner was missing — record with empty
				// mutex name so the count is still attributed.
				if trimmed == "" {
					continue
				}
				// Defensive: any non-blank line that wasn't the
				// partner and isn't another thread flushes the
				// pending record.
				siteCounts[siteKey{file: pendingFile, line: pendingLine, mutex: ""}]++
				havePending = false
			}
		case "TRANSACTIONS":
			if m := reHLL.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.HistoryListLength = v
			}
		case "BUFFER POOL AND MEMORY":
			if m := rePendingReads.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.PendingReads = v
			}
			if strings.HasPrefix(trimmed, "Pending writes:") {
				if m := rePendingWritesSum.FindStringSubmatch(trimmed); m != nil {
					lru, _ := strconv.Atoi(m[1])
					fl, _ := strconv.Atoi(m[2])
					sp, _ := strconv.Atoi(m[3])
					data.PendingWrites = lru + fl + sp
				}
			}
		case "INSERT BUFFER AND ADAPTIVE HASH INDEX":
			// The first Hash table size line gives the configured size;
			// overwrite on each so we end up with the last-seen value.
			if m := reHashTblSize.FindStringSubmatch(trimmed); m != nil {
				v, _ := strconv.Atoi(m[1])
				data.AHIActivity.HashTableSize = v
			}
			if m := reHashRate.FindStringSubmatch(trimmed); m != nil {
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

	// Flush any trailing pending record (file reached EOF with no mutex partner).
	if havePending {
		siteCounts[siteKey{file: pendingFile, line: pendingLine, mutex: ""}]++
	}

	data.SemaphoreCount = threadWaited

	// Materialise sites sorted desc by count, stable tie-break by
	// file ascending then line ascending so identical captures
	// render identically across runs.
	if len(siteCounts) > 0 {
		sites := make([]model.SemaphoreSite, 0, len(siteCounts))
		for k, c := range siteCounts {
			sites = append(sites, model.SemaphoreSite{
				File:      k.file,
				Line:      k.line,
				MutexName: k.mutex,
				WaitCount: c,
			})
		}
		sort.SliceStable(sites, func(i, j int) bool {
			if sites[i].WaitCount != sites[j].WaitCount {
				return sites[i].WaitCount > sites[j].WaitCount
			}
			if sites[i].File != sites[j].File {
				return sites[i].File < sites[j].File
			}
			if sites[i].Line != sites[j].Line {
				return sites[i].Line < sites[j].Line
			}
			return sites[i].MutexName < sites[j].MutexName
		})
		data.SemaphoreSites = sites
	}

	return data, diagnostics
}
