package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type projectVersion struct {
	Name     string `json:"name"`
	Released bool   `json:"released"`
}

// EnsureProjectFixVersionReleased returns an error unless Jira lists a version with the
// exact given name on the project (trimmed) and that version has released=true.
func (c *Client) EnsureProjectFixVersionReleased(ctx context.Context, projectKey, fixVersionName string) error {
	projectKey = strings.TrimSpace(projectKey)
	want := strings.TrimSpace(fixVersionName)
	if projectKey == "" {
		return fmt.Errorf("project key is required")
	}
	if want == "" {
		return fmt.Errorf("fix version name is required")
	}
	u := fmt.Sprintf("%s/rest/api/3/project/%s/versions", c.baseURL, url.PathEscape(projectKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build project versions request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira project versions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read project versions: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jira project versions %q returned %d: %s", projectKey, resp.StatusCode, truncate(string(body), 500))
	}

	var versions []projectVersion
	if err := json.Unmarshal(body, &versions); err != nil {
		return fmt.Errorf("parse project versions: %w", err)
	}
	for _, v := range versions {
		if strings.TrimSpace(v.Name) != want {
			continue
		}
		if !v.Released {
			return fmt.Errorf("fix version %q on project %q exists in Jira but is not marked Released", want, projectKey)
		}
		return nil
	}
	return fmt.Errorf("fix version %q was not found on Jira project %q", want, projectKey)
}
