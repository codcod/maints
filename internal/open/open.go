package open

import (
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

var jiraIssueKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-(\d+)$`)

// IssueBrowseURL returns the Jira web UI URL for a single issue (Cloud-style
// {base}/browse/KEY). baseURL is trimmed of trailing slashes.
func IssueBrowseURL(baseURL, key string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return base + "/browse/" + strings.TrimSpace(key)
}

// OpenBrowser opens the URL in the system default web browser.
func OpenBrowser(u string) error {
	if u == "" {
		return fmt.Errorf("empty url")
	}
	if parsed, err := url.Parse(u); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	} else if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must be http or https, got %q", parsed.Scheme)
	} else if parsed.Host == "" {
		return fmt.Errorf("url missing host")
	}
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// "start" is a built-in, not an executable; cmd.exe handles the URL.
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		c = exec.Command("open", u)
	default: // "linux" and *bsd
		c = exec.Command("xdg-open", u)
	}
	return c.Run()
}

// ValidateKey returns a canonical issue key or an error.
func ValidateKey(s string) (string, error) {
	k := strings.TrimSpace(s)
	if k == "" {
		return "", fmt.Errorf("empty issue key")
	}
	if jiraIssueKey.MatchString(k) {
		return k, nil
	}
	return "", fmt.Errorf("not a Jira issue key: %q (expected e.g. MAINT-123)", s)
}
