package dash

import (
	"reflect"
	"testing"
)

func colIDs(specs []columnSpec) []string {
	out := make([]string, len(specs))
	for i, c := range specs {
		out[i] = c.id
	}
	return out
}

func TestParseDashColumns(t *testing.T) {
	d, err := parseDashColumns("")
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != len(defaultDashColumnOrder) {
		t.Fatalf("default len %d", len(d))
	}
	if d[0].id != "key" || d[4].id != "summary" || d[4].summaryMax != 0 {
		t.Fatalf("default first/summary: %#v", d)
	}
	got, err := parseDashColumns("key, priority,due")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"key", "priority", "due"}; !reflect.DeepEqual(colIDs(got), want) {
		t.Fatalf("got %#v want %#v", colIDs(got), want)
	}
	_, err = parseDashColumns("nope")
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
	_, err = parseDashColumns("key,key")
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestParseDashColumnsSummaryWidth(t *testing.T) {
	got, err := parseDashColumns("KEY, SUMMARY[20], PRIORITY")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].id != "key" || got[1].id != "summary" || got[1].summaryMax != 20 || got[2].id != "priority" {
		t.Fatalf("specs: %#v", got)
	}
	if h := dashTableHeaders(got); h[1] != "SUMMARY[20]" {
		t.Fatalf("header[1] %q", h[1])
	}
	_, err = parseDashColumns("priority[5]")
	if err == nil {
		t.Fatal("expected error: only summary supports [N]")
	}
	_, err = parseDashColumns("summary[0]")
	if err == nil {
		t.Fatal("expected error for width 0")
	}
}

func TestParseDashColumnsCaseAndNames(t *testing.T) {
	got, err := parseDashColumns("KEY, Due, FIX_VERSION, SumMary")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"key", "due", "fixversion", "summary"}
	if !reflect.DeepEqual(colIDs(got), want) {
		t.Fatalf("got %v want %v", colIDs(got), want)
	}
}

func TestParseDashColumnsFixversionAliases(t *testing.T) {
	for _, s := range []string{"fixversion", "fix_version", "fix-version", " Fix Version "} {
		got, err := parseDashColumns("key, " + s)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if got[1].id != "fixversion" {
			t.Fatalf("%q: got[1]=%#v", s, got[1])
		}
	}
}
