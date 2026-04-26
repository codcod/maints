package release

import "testing"

func TestJqlStringLiteral(t *testing.T) {
	if s := jqlStringLiteral("plain"); s != `"plain"` {
		t.Fatalf("got %q", s)
	}
	inside := "a" + string('"') + "b"
	if got := jqlStringLiteral(inside); got != "\"a\\\"b\"" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultReleaseJQL(t *testing.T) {
	q := defaultReleaseJQL("DIG", "DS 2025.09.2")
	want := `project = DIG AND fixVersion = "DS 2025.09.2"`
	if q != want {
		t.Fatalf("got %q", q)
	}
}

func TestAssigneeAccountID(t *testing.T) {
	if x := assigneeAccountID(map[string]any{
		"assignee": map[string]any{"accountId": "abc-123"},
	}); x != "abc-123" {
		t.Fatalf("got %q", x)
	}
	if x := assigneeAccountID(map[string]any{"assignee": nil}); x != "" {
		t.Fatalf("unassigned: got %q", x)
	}
}

func TestIsDoneOrClosed(t *testing.T) {
	if !isDoneOrClosed("done") {
		t.Fatal("done")
	}
	if !isDoneOrClosed("Closed") {
		t.Fatal("closed")
	}
	if isDoneOrClosed("Open") {
		t.Fatal("open")
	}
}
