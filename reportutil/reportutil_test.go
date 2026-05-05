package reportutil

import (
	"math"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

func TestFormatNum(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"zero", 0, "0"},
		{"integer", 1234567, "1,234,567"},
		{"negative_integer", -9876543, "-9,876,543"},
		{"fraction", 12.345, "12.35"},
		{"small_fraction", 0.12345, "0.1235"},
		{"tiny_fraction", 0.000012345, "1.23e-05"},
		{"nan", math.NaN(), "NaN"},
		{"positive_infinity", math.Inf(1), "\u221e"},
		{"negative_infinity", math.Inf(-1), "-\u221e"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatNum(tc.in); got != tc.want {
				t.Fatalf("FormatNum(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"bytes", 900, "900 B"},
		{"kib", 900 * 1024, "900 KiB"},
		{"mib", 128 * 1024 * 1024, "128 MiB"},
		{"gib", 2560 * 1024 * 1024, "2.50 GiB"},
		{"tib", 3.25 * 1024 * 1024 * 1024 * 1024, "3.25 TiB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HumanBytes(tc.in); got != tc.want {
				t.Fatalf("HumanBytes(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestReportLookups(t *testing.T) {
	r := &model.Report{
		DBSection: &model.DBSection{
			Mysqladmin: &model.MysqladminData{
				Deltas: map[string][]float64{
					"Open_tables": {10, math.NaN(), 42, math.Inf(1)},
					"Questions":   {0, 1, 2},
				},
				IsCounter: map[string]bool{
					"Questions": true,
				},
			},
		},
		VariablesSection: &model.VariablesSection{
			PerSnapshot: []model.SnapshotVariables{
				{
					Data: &model.VariablesData{Entries: []model.VariableEntry{
						{Name: "table_open_cache", Value: "1000"},
					}},
				},
				{
					Data: &model.VariablesData{Entries: []model.VariableEntry{
						{Name: "table_open_cache", Value: " 2000 "},
						{Name: "null_value", Value: "NULL"},
					}},
				},
			},
		},
	}

	if got, ok := GaugeLast(r, "Open_tables"); !ok || got != 42 {
		t.Fatalf("GaugeLast Open_tables = %v, %v; want 42, true", got, ok)
	}
	if _, ok := GaugeLast(r, "Questions"); ok {
		t.Fatal("GaugeLast returned ok for a counter")
	}
	if got, ok := VariableFloat(r, "TABLE_OPEN_CACHE"); !ok || got != 2000 {
		t.Fatalf("VariableFloat table_open_cache = %v, %v; want 2000, true", got, ok)
	}
	if _, ok := VariableFloat(r, "null_value"); ok {
		t.Fatal("VariableFloat returned ok for NULL")
	}
}
