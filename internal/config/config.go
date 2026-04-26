package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	JiraURL      string
	JiraUsername string
	JiraAPIToken string
	CursorAPIKey string
}

// Load reads configuration from a .env file (if present) and environment variables.
// Environment variables take precedence over values in the .env file.
func Load() (*Config, error) {
	// Best-effort load of .env; ignore if file doesn't exist
	_ = godotenv.Load()

	cfg := &Config{
		JiraURL:      os.Getenv("JIRA_URL"),
		JiraUsername: os.Getenv("JIRA_USERNAME"),
		JiraAPIToken: os.Getenv("JIRA_API_TOKEN"),
		CursorAPIKey: os.Getenv("CURSOR_API_KEY"),
	}

	return cfg, cfg.validate()
}

// LoadJiraURLOnly loads .env (if present) and returns the Jira base URL when
// JIRA_URL is set. Use for commands that only need to construct browse links
// (e.g. maints open) — it does not require JIRA_USERNAME or JIRA_API_TOKEN.
func LoadJiraURLOnly() (string, error) {
	_ = godotenv.Load()
	u := strings.TrimSpace(os.Getenv("JIRA_URL"))
	if u == "" {
		return "", fmt.Errorf("JIRA_URL is required (set in the environment or .env)")
	}
	return u, nil
}

// LoadJiraOnly reads the same sources as Load but validates only Jira credentials.
// Use for commands that do not need CURSOR_API_KEY (e.g. maints dig, maints dash).
func LoadJiraOnly() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		JiraURL:      os.Getenv("JIRA_URL"),
		JiraUsername: os.Getenv("JIRA_USERNAME"),
		JiraAPIToken: os.Getenv("JIRA_API_TOKEN"),
	}
	return cfg, cfg.validateJiraOnly()
}

func (c *Config) validateJiraOnly() error {
	var missing []string
	if c.JiraURL == "" {
		missing = append(missing, "JIRA_URL")
	}
	if c.JiraUsername == "" {
		missing = append(missing, "JIRA_USERNAME")
	}
	if c.JiraAPIToken == "" {
		missing = append(missing, "JIRA_API_TOKEN")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables:\n  %s", strings.Join(missing, "\n  "))
	}
	return nil
}

func (c *Config) validate() error {
	var missing []string
	if c.JiraURL == "" {
		missing = append(missing, "JIRA_URL")
	}
	if c.JiraUsername == "" {
		missing = append(missing, "JIRA_USERNAME")
	}
	if c.JiraAPIToken == "" {
		missing = append(missing, "JIRA_API_TOKEN")
	}
	if c.CursorAPIKey == "" {
		missing = append(missing, "CURSOR_API_KEY")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables:\n  %s", strings.Join(missing, "\n  "))
	}
	return nil
}
