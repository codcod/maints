package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GetIssueFieldsMap returns the issue's "fields" object for a comma-separated field list
// (e.g. "fixVersions,issuelinks").
func (c *Client) GetIssueFieldsMap(ctx context.Context, key, fieldList string) (map[string]any, error) {
	escaped := url.PathEscape(strings.TrimSpace(key))
	u := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=%s", c.baseURL, escaped, fieldList)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira API returned %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	var raw struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.Fields == nil {
		return nil, nil
	}
	return raw.Fields, nil
}

// UpdateIssue sets issue fields in one PUT. fields is the "fields" object (not wrapped).
func (c *Client) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	escaped := url.PathEscape(strings.TrimSpace(key))
	u := c.baseURL + "/rest/api/3/issue/" + escaped
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	rbody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jira put issue %d: %s", resp.StatusCode, truncate(string(rbody), 800))
	}
	return nil
}

// AddIssueComment posts an Atlassian Document Format (ADF) body to the issue.
func (c *Client) AddIssueComment(ctx context.Context, key string, body map[string]any) error {
	escaped := url.PathEscape(strings.TrimSpace(key))
	u := c.baseURL + "/rest/api/3/issue/" + escaped + "/comment"
	payload, err := json.Marshal(map[string]any{"body": body})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	rbody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("jira add comment %d: %s", resp.StatusCode, truncate(string(rbody), 800))
	}
	return nil
}
