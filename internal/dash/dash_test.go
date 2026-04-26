package dash

import "testing"

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
