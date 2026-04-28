package dash

import (
	"strings"
	"testing"
	"time"
)

func TestJiraAssigneeJQLString(t *testing.T) {
	if got, want := jiraAssigneeJQLString("x"), `"x"`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// a, double-quote, b, backslash — verify escaping round-trip shape
	in := "a\x22b\x5c"
	got := jiraAssigneeJQLString(in)
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("expected wrapped in quotes: %q", got)
	}
	if !strings.Contains(got, `\\`) || !strings.Contains(got, `\"`) {
		t.Fatalf("expected JQL-escaped: %q", got)
	}
}

func TestIsMaintUrgentStatus(t *testing.T) {
	for _, s := range []string{"Open", " open ", "AWAITING INPUT", "TRIAGE", "triage"} {
		if !isMaintUrgentStatus(s) {
			t.Fatalf("expected urgent: %q", s)
		}
	}
	for _, s := range []string{"", "Done", "In Progress", "Closed"} {
		if isMaintUrgentStatus(s) {
			t.Fatalf("expected not urgent: %q", s)
		}
	}
}

func TestIsPastDueCell(t *testing.T) {
	if isPastDueCell("—") {
		t.Fatal("placeholder")
	}
	pastYMD := time.Date(2010, 1, 2, 0, 0, 0, 0, time.Local)
	if isPastDueCell(pastYMD.Format("2006-01-02")+"T12:00:00.000+0000") != true {
		t.Fatal("date with time part")
	}
	if isPastDueCell("2099-12-01") {
		t.Fatal("future not past")
	}
}

func TestEffectiveDashJQL(t *testing.T) {
	j, err := effectiveDashJQL(Options{JQL: "foo"})
	if err != nil || j != "foo" {
		t.Fatalf("jql: %q %v", j, err)
	}
	_, err = effectiveDashJQL(Options{JQL: "a", Assignee: "b"})
	if err == nil || !strings.Contains(err.Error(), "either --jql or --assignee") {
		t.Fatalf("expected mutual exclusion: %v", err)
	}
	j, err = effectiveDashJQL(Options{Supervisor: true})
	if err != nil || j != DefaultJQLSupervisor {
		t.Fatalf("supervisor: %q %v", j, err)
	}
	_, err = effectiveDashJQL(Options{JQL: "x", Supervisor: true})
	if err == nil || !strings.Contains(err.Error(), "either --jql or --supervisor") {
		t.Fatalf("expected jql+supervisor exclusion: %v", err)
	}
	_, err = effectiveDashJQL(Options{Assignee: "u@x.com", Supervisor: true})
	if err == nil || !strings.Contains(err.Error(), "either --assignee or --supervisor") {
		t.Fatalf("expected assignee+supervisor exclusion: %v", err)
	}
}
