package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateIssue_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/X-1" || r.Method != http.MethodPut {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		fields, _ := body["fields"].(map[string]any)
		if fields == nil {
			t.Fatal("no fields")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	if err := c.UpdateIssue(context.Background(), "X-1", map[string]any{"summary": "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestAddIssueComment_201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/X-1/comment" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	adf := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "hi"}}}},
	}
	if err := c.AddIssueComment(context.Background(), "X-1", adf); err != nil {
		t.Fatal(err)
	}
}
