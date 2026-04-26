package config

import (
	"strings"
	"testing"
)

func TestValidateJiraOnly(t *testing.T) {
	t.Run("all jira fields present", func(t *testing.T) {
		cfg := Config{
			JiraURL:      "https://example.atlassian.net",
			JiraUsername: "user@example.com",
			JiraAPIToken: "token123",
		}
		if err := cfg.validateJiraOnly(); err != nil {
			t.Fatalf("validateJiraOnly() = %v", err)
		}
	})
	t.Run("missing token", func(t *testing.T) {
		cfg := Config{
			JiraURL:      "https://example.atlassian.net",
			JiraUsername: "user@example.com",
		}
		err := cfg.validateJiraOnly()
		if err == nil || !strings.Contains(err.Error(), "JIRA_API_TOKEN") {
			t.Fatalf("expected error mentioning JIRA_API_TOKEN, got %v", err)
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		missing []string
	}{
		{
			name: "all fields present",
			cfg: Config{
				JiraURL:      "https://example.atlassian.net",
				JiraUsername: "user@example.com",
				JiraAPIToken: "token123",
				CursorAPIKey: "cursor-key",
			},
			wantErr: false,
		},
		{
			name:    "all fields empty",
			cfg:     Config{},
			wantErr: true,
			missing: []string{"JIRA_URL", "JIRA_USERNAME", "JIRA_API_TOKEN", "CURSOR_API_KEY"},
		},
		{
			name: "only JIRA_URL missing",
			cfg: Config{
				JiraUsername: "user@example.com",
				JiraAPIToken: "token123",
				CursorAPIKey: "cursor-key",
			},
			wantErr: true,
			missing: []string{"JIRA_URL"},
		},
		{
			name: "multiple fields missing",
			cfg: Config{
				JiraURL: "https://example.atlassian.net",
			},
			wantErr: true,
			missing: []string{"JIRA_USERNAME", "JIRA_API_TOKEN", "CURSOR_API_KEY"},
		},
		{
			name: "only CURSOR_API_KEY missing",
			cfg: Config{
				JiraURL:      "https://example.atlassian.net",
				JiraUsername: "user@example.com",
				JiraAPIToken: "token123",
			},
			wantErr: true,
			missing: []string{"CURSOR_API_KEY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				for _, field := range tt.missing {
					if !strings.Contains(err.Error(), field) {
						t.Errorf("expected error to mention %q, got: %v", field, err)
					}
				}
			}
		})
	}
}
