package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssigneeString(t *testing.T) {
	if got := AssigneeString(map[string]any{"displayName": "  Alex  "}); got != "Alex" {
		t.Fatalf("displayName: %q", got)
	}
	if got := AssigneeString(map[string]any{"emailAddress": "a@b.co"}); got != "a@b.co" {
		t.Fatalf("email: %q", got)
	}
	if got := AssigneeString(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}

func TestGetIssueFields_issuelinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/MAINT-1" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("fields") == "" {
			t.Error("expected fields= query")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":{"issuelinks":[]}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "u", "t")
	m, err := c.GetIssueFields(context.Background(), "MAINT-1", []string{"issuelinks"})
	if err != nil {
		t.Fatal(err)
	}
	if m["issuelinks"] == nil {
		t.Fatal("expected issuelinks key")
	}
}
