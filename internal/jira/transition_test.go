package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListTransitions_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/X-1/transitions" || r.Method != http.MethodGet {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transitions": []any{
				map[string]any{
					"id":   "31",
					"name": "Mark Done",
					"to":   map[string]any{"name": "Done"},
					"fields": map[string]any{
						"resolution": map[string]any{"required": true},
					},
				},
			},
		})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	tlist, err := c.ListTransitions(context.Background(), "X-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tlist) != 1 || tlist[0].ID != "31" || tlist[0].ToStatusName != "Done" {
		t.Fatalf("got %#v", tlist)
	}
}

func TestPostTransition_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/X-1/transitions" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	if err := c.PostTransition(context.Background(), "X-1", "5", nil); err != nil {
		t.Fatal(err)
	}
}

func TestTransitionToStatusName_usesOK(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"transitions": []any{
					map[string]any{
						"id": "9",
						"to": map[string]any{"name": "Done"},
					},
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			n++
			w.WriteHeader(http.StatusOK) // Jira can return 200
			return
		}
		t.Fatalf("bad %s", r.Method)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	if err := c.TransitionToStatusName(context.Background(), "X-1", "Done", "Done"); err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("no POST")
	}
}
