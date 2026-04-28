package dash

import (
	"strings"
	"testing"
	"time"

	"github.com/codcod/maints-triage/internal/jira"
)

func TestBuildRowsSkipDig(t *testing.T) {
	hits := []jira.IssueJQL{{
		Key: "MAINT-1",
		Fields: map[string]any{
			"summary": "x",
			"issuelinks": []any{
				map[string]any{
					"type":         map[string]any{"name": "Solved by"},
					"inwardIssue":  map[string]any{"key": "MAINT-1"},
					"outwardIssue": map[string]any{"key": "DIG-1"},
				},
			},
		},
	}}
	rowsWithDig, err := buildRows(hits, "DIG", "Solved by", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rowsWithDig) != 1 || len(rowsWithDig[0].DIGs) == 0 {
		t.Fatalf("expected DIG sub-rows: %+v", rowsWithDig)
	}
	rowsNoDig, err := buildRows(hits, "DIG", "Solved by", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rowsNoDig) != 1 || len(rowsNoDig[0].DIGs) != 0 {
		t.Fatalf("expected no DIGs: %+v", rowsNoDig)
	}
}

func TestPickDIGEnd(t *testing.T) {
	maint := "MAINT-1"
	digProj := "DIG"
	t.Run("outward DIG inward MAINT (dig command default)", func(t *testing.T) {
		link := map[string]any{
			"inwardIssue":  map[string]any{"key": "MAINT-1"},
			"outwardIssue": map[string]any{"key": "DIG-99"},
		}
		k, side, ok := pickDIGEnd(maint, link, digProj)
		if !ok || k != "DIG-99" || side != "outwardIssue" {
			t.Fatalf("got %q %q %v", k, side, ok)
		}
	})
	t.Run("opposite ends", func(t *testing.T) {
		link := map[string]any{
			"outwardIssue": map[string]any{"key": "MAINT-1"},
			"inwardIssue":  map[string]any{"key": "DIG-2"},
		}
		k, side, ok := pickDIGEnd(maint, link, digProj)
		if !ok || k != "DIG-2" || side != "inwardIssue" {
			t.Fatalf("got %q %q %v", k, side, ok)
		}
	})
	t.Run("no DIG", func(t *testing.T) {
		link := map[string]any{
			"inwardIssue":  map[string]any{"key": "MAINT-1"},
			"outwardIssue": map[string]any{"key": "MAINT-2"},
		}
		_, _, ok := pickDIGEnd(maint, link, digProj)
		if ok {
			t.Fatal("expected false")
		}
	})
}

func TestPickDIGEnd_omitsCurrentIssueInward(t *testing.T) {
	// Jira can omit the viewed issue: only the remote end (DIG) is present.
	link := map[string]any{
		"inwardIssue":  nil,
		"outwardIssue": map[string]any{"key": "DIG-1"},
	}
	k, side, ok := pickDIGEnd("MAINT-1", link, "DIG")
	if !ok || k != "DIG-1" || side != "outwardIssue" {
		t.Fatalf("got %q %q %v", k, side, ok)
	}
}

func TestLinkTypeMatch(t *testing.T) {
	want := "Solved by"
	t.Run("name only exact", func(t *testing.T) {
		if !linkTypeMatch(want, map[string]any{
			"name": "Solved by",
		}) {
			t.Fatal("name EqualFold")
		}
	})
	t.Run("UI label in inward, short name in name", func(t *testing.T) {
		// Jira: type.name is often a short id; the “Solved by” wording is on inward/outward.
		if !linkTypeMatch(want, map[string]any{
			"name":    "Solves",
			"inward":  "is solved by",
			"outward": "solves",
		}) {
			t.Fatal("inward substring should match Solved by")
		}
	})
	t.Run("unrelated", func(t *testing.T) {
		if linkTypeMatch(want, map[string]any{
			"name": "Blocks",
		}) {
			t.Fatal("no match")
		}
	})
}

func TestMaintNeedsAttention(t *testing.T) {
	if !maintNeedsAttention(Row{Priority: "Critical", Status: "Scheduled", Due: "2099-01-01"}) {
		t.Fatal("Critical should need attention")
	}
	if !maintNeedsAttention(Row{Priority: "Minor", Status: "Open", Due: "2099-01-01"}) {
		t.Fatal("Open status should need attention")
	}
	past := time.Date(2010, 1, 2, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	if !maintNeedsAttention(Row{Priority: "Minor", Status: "In Progress", Due: past}) {
		t.Fatal("Past due should need attention")
	}
	if maintNeedsAttention(Row{Priority: "Minor", Status: "In Progress", Due: "2099-01-01"}) {
		t.Fatal("clean row should not need attention")
	}
}

func TestPrintSupervisorSummary(t *testing.T) {
	past := time.Date(2010, 1, 2, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	rows := []Row{
		{Priority: "Critical", Status: "Scheduled", Due: "2099-01-01", Assignee: "Alice"},
		{Priority: "Minor", Status: "Open", Due: "2099-01-01", Assignee: "Bob"},
		{Priority: "Minor", Status: "In Progress", Due: past, Assignee: "Alice"},
	}
	var buf strings.Builder
	printSupervisorSummary(&buf, rows, false)
	out := buf.String()
	if !strings.Contains(out, "Total issues : 3") {
		t.Fatalf("missing total: %q", out)
	}
	if !strings.Contains(out, "Needs action : 3") {
		t.Fatalf("missing needs action: %q", out)
	}
	if !strings.Contains(out, "Critical/Blocker 1") || !strings.Contains(out, "Past due 1") || !strings.Contains(out, "Urgent status 1") {
		t.Fatalf("expected breakdown: %q", out)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "2 total") || !strings.Contains(out, "2 needs action") {
		t.Fatalf("expected Alice aggregate: %q", out)
	}
	if !strings.Contains(out, "Open: 1") || !strings.Contains(out, "In Progress: 1") {
		t.Fatalf("expected status line: %q", out)
	}
}
