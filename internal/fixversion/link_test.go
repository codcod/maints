package fixversion

import "testing"

func TestFindLinkedMaints(t *testing.T) {
	dk := "DIG-1"
	maintP := "MAINT"
	t.Run("inward maint outward dig", func(t *testing.T) {
		links := []any{
			map[string]any{
				"type":         map[string]any{"name": "Solved by", "inward": "is solved by", "outward": "solves"},
				"inwardIssue":  map[string]any{"key": "MAINT-2"},
				"outwardIssue": map[string]any{"key": "DIG-1"},
			},
		}
		ks := findLinkedMaints(links, dk, "Solved by", maintP)
		if len(ks) != 1 || ks[0] != "MAINT-2" {
			t.Fatalf("got %v", ks)
		}
	})
	t.Run("opposite", func(t *testing.T) {
		links := []any{
			map[string]any{
				"type":         map[string]any{"name": "Solved by"},
				"inwardIssue":  map[string]any{"key": "DIG-1"},
				"outwardIssue": map[string]any{"key": "MAINT-3"},
			},
		}
		ks := findLinkedMaints(links, dk, "Solved by", maintP)
		if len(ks) != 1 || ks[0] != "MAINT-3" {
			t.Fatalf("got %v", ks)
		}
	})
	t.Run("omitted inward (view MAINT, remote is DIG) — from DIG: MAINT is outward", func(t *testing.T) {
		// Same shape as in dash: inward absent, other end is MAINT when the viewed issue is DIG-1? Here we
		// simulate the symmetric case: inward null, outward MAINT-9, current issue implied inward = DIG-1.
		links := []any{
			map[string]any{
				"type":         map[string]any{"name": "Solved by", "inward": "is solved by"},
				"outwardIssue": map[string]any{"key": "MAINT-9"},
			},
		}
		ks := findLinkedMaints(links, dk, "Solved by", maintP)
		if len(ks) != 1 || ks[0] != "MAINT-9" {
			t.Fatalf("got %v", ks)
		}
	})
	t.Run("omitted outward (current is DIG, inward is MAINT)", func(t *testing.T) {
		links := []any{
			map[string]any{
				"type":         map[string]any{"name": "Solved by", "inward": "is solved by"},
				"inwardIssue":  map[string]any{"key": "MAINT-10"},
				"outwardIssue": nil,
			},
		}
		ks := findLinkedMaints(links, dk, "Solved by", maintP)
		if len(ks) != 1 || ks[0] != "MAINT-10" {
			t.Fatalf("got %v", ks)
		}
	})
}

func TestLinkTypeMatch(t *testing.T) {
	if !linkTypeMatch("Solved by", map[string]any{
		"name":    "Solves",
		"inward":  "is solved by",
		"outward": "solves",
	}) {
		t.Fatal("expected Solved by to match Solves / is solved by")
	}
}
