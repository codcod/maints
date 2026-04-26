package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssueKeysFromJQLSearchPayload(t *testing.T) {
	t.Run("array issues", func(t *testing.T) {
		got := issueKeysFromJQLSearchPayload(map[string]any{
			"issues": []any{
				map[string]any{"key": "MAINT-1"},
				map[string]any{"key": "  MAINT-2  "},
			},
		})
		if len(got) != 2 || got[0] != "MAINT-1" || got[1] != "MAINT-2" {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("issues nodes wrapper", func(t *testing.T) {
		got := issueKeysFromJQLSearchPayload(map[string]any{
			"issues": map[string]any{
				"nodes": []any{
					map[string]any{"key": "DIG-9"},
				},
			},
		})
		if len(got) != 1 || got[0] != "DIG-9" {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("nil issues", func(t *testing.T) {
		got := issueKeysFromJQLSearchPayload(map[string]any{})
		if len(got) != 0 {
			t.Fatalf("got %#v", got)
		}
	})
}

func TestSearchIssueKeysPOST_pagination(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/search/jql" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issues":        []any{map[string]any{"key": "A-1"}},
				"isLast":        false,
				"nextPageToken": "tok2",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{map[string]any{"key": "A-2"}},
			"isLast": true,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "u", "t")
	keys, err := c.SearchIssueKeysPOST(context.Background(), "project = A")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
	if len(keys) != 2 || keys[0] != "A-1" || keys[1] != "A-2" {
		t.Fatalf("keys %#v", keys)
	}
}

func TestCreateIssueLink_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issueLink" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "u", "t")
	if err := c.CreateIssueLink(context.Background(), "Relates", "DIG-1", "MAINT-1"); err != nil {
		t.Fatal(err)
	}
}
