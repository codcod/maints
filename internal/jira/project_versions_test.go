package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureProjectFixVersionReleased_ok(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/project/DIG/versions" || r.Method != http.MethodGet {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"name":"1.0","released":true},
			{"name":"DS 2025.09.2","released":true}
		]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	if err := c.EnsureProjectFixVersionReleased(context.Background(), "DIG", "DS 2025.09.2"); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureProjectFixVersionReleased_notReleased(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"DS 2025.09.2","released":false}]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	err := c.EnsureProjectFixVersionReleased(context.Background(), "DIG", "DS 2025.09.2")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestEnsureProjectFixVersionReleased_missing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"other","released":true}]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "a", "b")
	err := c.EnsureProjectFixVersionReleased(context.Background(), "DIG", "DS 2025.09.2")
	if err == nil {
		t.Fatal("want error")
	}
}
