package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"exceeds limit", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"zero limit", "hi", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"nil input", nil, ""},
		{"plain string", "hello", "hello"},
		{"empty string", "", ""},
		{"integer fallback", 42, "42"},
		{
			name: "ADF paragraph with text",
			input: map[string]any{
				"type": "doc",
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{"type": "text", "text": "Hello, world!"},
						},
					},
				},
			},
			want: "Hello, world!\n",
		},
		{
			name: "ADF node with direct text field",
			input: map[string]any{
				"type": "text",
				"text": "direct text",
			},
			want: "direct text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.input)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractADF(t *testing.T) {
	tests := []struct {
		name string
		node map[string]any
		want string
	}{
		{
			name: "empty node",
			node: map[string]any{},
			want: "",
		},
		{
			name: "node with direct text",
			node: map[string]any{"text": "direct"},
			want: "direct",
		},
		{
			name: "paragraph adds trailing newline",
			node: map[string]any{
				"type": "doc",
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{"type": "text", "text": "line one"},
						},
					},
				},
			},
			want: "line one\n",
		},
		{
			name: "multiple block nodes each get trailing newlines",
			node: map[string]any{
				"content": []any{
					map[string]any{
						"type":    "heading",
						"content": []any{map[string]any{"type": "text", "text": "Title"}},
					},
					map[string]any{
						"type":    "paragraph",
						"content": []any{map[string]any{"type": "text", "text": "Body"}},
					},
				},
			},
			want: "Title\nBody\n",
		},
		{
			name: "inline node does not add newline",
			node: map[string]any{
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "inline",
					},
				},
			},
			want: "inline",
		},
		{
			name: "nested content is flattened",
			node: map[string]any{
				"content": []any{
					map[string]any{
						"type": "bulletList",
						"content": []any{
							map[string]any{
								"type": "listItem",
								"content": []any{
									map[string]any{
										"type":    "paragraph",
										"content": []any{map[string]any{"type": "text", "text": "item"}},
									},
								},
							},
						},
					},
				},
			},
			want: "item\n\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractADF(tt.node)
			if got != tt.want {
				t.Errorf("extractADF() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.atlassian.net/", "user@example.com", "mytoken")

	if c.baseURL != "https://example.atlassian.net" {
		t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
	}
	if c.authHeader == "" {
		t.Error("authHeader should not be empty")
	}
	if c.http == nil {
		t.Error("http client should not be nil")
	}
}

func TestFetchIssue(t *testing.T) {
	t.Run("successful fetch with all fields", func(t *testing.T) {
		payload := map[string]any{
			"key": "MAINT-123",
			"fields": map[string]any{
				"summary": "Test issue summary",
				"description": map[string]any{
					"type": "doc",
					"content": []any{
						map[string]any{
							"type": "paragraph",
							"content": []any{
								map[string]any{"type": "text", "text": "Issue description"},
							},
						},
					},
				},
				"status":   map[string]any{"name": "Open"},
				"priority": map[string]any{"name": "High"},
				"reporter": map[string]any{"displayName": "Alice", "emailAddress": "alice@example.com"},
				"assignee": map[string]any{"displayName": "Bob"},
				"components": []any{
					map[string]any{"name": "Backend"},
				},
				"versions":    []any{map[string]any{"name": "1.0"}},
				"fixVersions": []any{map[string]any{"name": "1.1"}},
				"labels":      []any{"bug", "urgent"},
				"comment": map[string]any{
					"comments": []any{
						map[string]any{
							"author":  map[string]any{"displayName": "Charlie"},
							"created": "2024-01-15T10:00:00.000Z",
							"body":    "First comment",
						},
					},
				},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user", "token")
		issue, err := client.FetchIssue("MAINT-123")
		if err != nil {
			t.Fatalf("FetchIssue() unexpected error: %v", err)
		}

		if issue.Key != "MAINT-123" {
			t.Errorf("Key = %q, want %q", issue.Key, "MAINT-123")
		}
		if issue.Summary != "Test issue summary" {
			t.Errorf("Summary = %q, want %q", issue.Summary, "Test issue summary")
		}
		if issue.Status != "Open" {
			t.Errorf("Status = %q, want %q", issue.Status, "Open")
		}
		if issue.Priority != "High" {
			t.Errorf("Priority = %q, want %q", issue.Priority, "High")
		}
		if issue.Reporter != "Alice" {
			t.Errorf("Reporter = %q, want %q", issue.Reporter, "Alice")
		}
		if issue.Assignee != "Bob" {
			t.Errorf("Assignee = %q, want %q", issue.Assignee, "Bob")
		}
		if len(issue.Components) != 1 || issue.Components[0] != "Backend" {
			t.Errorf("Components = %v, want [Backend]", issue.Components)
		}
		if len(issue.AffectedVersions) != 1 || issue.AffectedVersions[0] != "1.0" {
			t.Errorf("AffectedVersions = %v, want [1.0]", issue.AffectedVersions)
		}
		if len(issue.FixVersions) != 1 || issue.FixVersions[0] != "1.1" {
			t.Errorf("FixVersions = %v, want [1.1]", issue.FixVersions)
		}
		if len(issue.Labels) != 2 {
			t.Errorf("Labels = %v, want 2 labels", issue.Labels)
		}
		if len(issue.Comments) != 1 {
			t.Fatalf("Comments len = %d, want 1", len(issue.Comments))
		}
		if issue.Comments[0].Author != "Charlie" {
			t.Errorf("Comment author = %q, want %q", issue.Comments[0].Author, "Charlie")
		}
		if issue.Comments[0].Body != "First comment" {
			t.Errorf("Comment body = %q, want %q", issue.Comments[0].Body, "First comment")
		}
	})

	t.Run("auth header is set on request", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"key": "X-1", "fields": map[string]any{}})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user", "secret")
		_, _ = client.FetchIssue("X-1")

		if gotAuth == "" || gotAuth[:6] != "Basic " {
			t.Errorf("expected Basic auth header, got %q", gotAuth)
		}
	})

	t.Run("404 response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user", "token")
		_, err := client.FetchIssue("MISSING-1")
		if err == nil {
			t.Error("FetchIssue() should return error for 404 response")
		}
	})

	t.Run("invalid JSON response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`not valid json`))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "user", "token")
		_, err := client.FetchIssue("MAINT-1")
		if err == nil {
			t.Error("FetchIssue() should return error for invalid JSON")
		}
	})
}
