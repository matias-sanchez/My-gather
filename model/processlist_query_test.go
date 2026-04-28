package model

import (
	"strings"
	"testing"
	"time"
)

func TestObservedProcesslistQueryFingerprintGroupsLiteralVariants(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	first, ok := NewObservedProcesslistQuery(ts, "app", "shop", "Query", "Sending data",
		"SELECT * FROM orders WHERE id = 123 AND email = 'a@example.com'", 1200, true, 10, true, 2, true)
	if !ok {
		t.Fatal("first query unexpectedly ineligible")
	}
	second, ok := NewObservedProcesslistQuery(ts.Add(time.Second), "app", "shop", "Query", "Sending data",
		"select *  from orders where id = 456 and email = 'b@example.com'", 2500, true, 30, true, 4, true)
	if !ok {
		t.Fatal("second query unexpectedly ineligible")
	}

	merged := MergeObservedProcesslistQueries([]ObservedProcesslistQuery{first, second})
	if got, want := len(merged), 1; got != want {
		t.Fatalf("merged len = %d, want %d: %#v", got, want, merged)
	}
	got := merged[0]
	if got.Fingerprint == "" || !strings.HasPrefix(got.Fingerprint, "q_") {
		t.Fatalf("Fingerprint = %q, want stable q_ prefix", got.Fingerprint)
	}
	if got.SeenSamples != 2 {
		t.Errorf("SeenSamples = %d, want 2", got.SeenSamples)
	}
	if got.MaxTimeMS != 2500 {
		t.Errorf("MaxTimeMS = %v, want 2500", got.MaxTimeMS)
	}
	if got.MaxRowsExamined != 30 {
		t.Errorf("MaxRowsExamined = %v, want 30", got.MaxRowsExamined)
	}
	if got.MaxRowsSent != 4 {
		t.Errorf("MaxRowsSent = %v, want 4", got.MaxRowsSent)
	}
}

func TestObservedProcesslistQueryFingerprintKeepsBacktickIdentifiers(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	orders, ok := NewObservedProcesslistQuery(ts, "app", "shop", "Query", "Sending data",
		"SELECT * FROM `orders` WHERE id = 123", 1200, true, 10, true, 2, true)
	if !ok {
		t.Fatal("orders query unexpectedly ineligible")
	}
	customers, ok := NewObservedProcesslistQuery(ts.Add(time.Second), "app", "shop", "Query", "Sending data",
		"SELECT * FROM `customers` WHERE id = 456", 2500, true, 30, true, 4, true)
	if !ok {
		t.Fatal("customers query unexpectedly ineligible")
	}

	merged := MergeObservedProcesslistQueries([]ObservedProcesslistQuery{orders, customers})
	if got, want := len(merged), 2; got != want {
		t.Fatalf("merged len = %d, want %d: %#v", got, want, merged)
	}
	if orders.Fingerprint == customers.Fingerprint {
		t.Fatalf("backtick-quoted identifiers collapsed into one fingerprint %q", orders.Fingerprint)
	}
	if !strings.Contains(orders.Snippet, "`orders`") {
		t.Fatalf("orders snippet = %q, want quoted identifier preserved", orders.Snippet)
	}
	if !strings.Contains(customers.Snippet, "`customers`") {
		t.Fatalf("customers snippet = %q, want quoted identifier preserved", customers.Snippet)
	}
}

func TestObservedProcesslistQueryExcludesIdleAndBoundsSnippet(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		command string
		info    string
	}{
		{name: "sleep", command: "Sleep", info: "select 1"},
		{name: "daemon", command: "Daemon", info: "select 1"},
		{name: "empty command", command: "", info: "select 1"},
		{name: "null info", command: "Query", info: "NULL"},
		{name: "empty info", command: "Query", info: "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := NewObservedProcesslistQuery(ts, "app", "shop", tc.command, "state", tc.info, 1, true, 0, false, 0, false); ok {
				t.Fatalf("NewObservedProcesslistQuery returned eligible row: %+v", got)
			}
		})
	}

	longInfo := "select " + strings.Repeat("very_long_column_name, ", MaxObservedProcesslistQuerySnippetRunes)
	got, ok := NewObservedProcesslistQuery(ts, "app", "shop", "Query", "Sending data", longInfo, 1, true, 0, false, 0, false)
	if !ok {
		t.Fatal("long query unexpectedly ineligible")
	}
	if len([]rune(got.Snippet)) > MaxObservedProcesslistQuerySnippetRunes {
		t.Fatalf("snippet rune len = %d, want <= %d", len([]rune(got.Snippet)), MaxObservedProcesslistQuerySnippetRunes)
	}
	if !strings.HasSuffix(got.Snippet, "...") {
		t.Fatalf("truncated snippet = %q, want trailing ...", got.Snippet)
	}
}

func TestMergeObservedProcesslistQueriesSortsAndBounds(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	var rows []ObservedProcesslistQuery
	for i := 0; i < MaxObservedProcesslistQueries+3; i++ {
		q, ok := NewObservedProcesslistQuery(ts.Add(time.Duration(i)*time.Second), "app", "shop", "Query", "Sending data",
			"select * from t where id = "+string(rune('a'+i)), float64(i+1)*1000, true, float64(i), true, 0, true)
		if !ok {
			t.Fatalf("query %d unexpectedly ineligible", i)
		}
		rows = append(rows, q)
	}

	merged := MergeObservedProcesslistQueries(rows)
	if got, want := len(merged), MaxObservedProcesslistQueries; got != want {
		t.Fatalf("merged len = %d, want %d", got, want)
	}
	for i := 1; i < len(merged); i++ {
		if merged[i-1].MaxTimeMS < merged[i].MaxTimeMS {
			t.Fatalf("merged not sorted by age descending at %d: %v < %v", i, merged[i-1].MaxTimeMS, merged[i].MaxTimeMS)
		}
	}
}
