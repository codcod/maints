package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetMyselfAccountID_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" || r.Method != http.MethodGet {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"accountId":"u-1"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	id, err := c.GetMyselfAccountID(context.Background())
	if err != nil || id != "u-1" {
		t.Fatalf("got %q %v", id, err)
	}
}
