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

// BaseURL returns the normalized Jira site URL (no trailing slash).
func (c *Client) BaseURL() string {
	return c.baseURL
}

// SearchIssueKeysPOST runs JQL via POST /rest/api/3/search/jql and returns every
// matching issue key (paginated with nextPageToken), matching jira_dupe.py behavior.
func (c *Client) SearchIssueKeysPOST(ctx context.Context, jql string) ([]string, error) {
	const pageSize = 50
	var all []string
	var nextToken string
	for {
		payload := map[string]any{
			"jql":        jql,
			"maxResults": pageSize,
			"fields":     []string{"key"},
		}
		if nextToken != "" {
			payload["nextPageToken"] = nextToken
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal search body: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/api/3/search/jql", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build search request: %w", err)
		}
		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Accept", contentTypeJSON)
		req.Header.Set("Content-Type", contentTypeJSON)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute search request: %w", err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read search response: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jira search/jql returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
		}

		var data map[string]any
		if err := json.Unmarshal(respBody, &data); err != nil {
			return nil, fmt.Errorf("parse search response: %w", err)
		}

		keys := issueKeysFromJQLSearchPayload(data)
		for _, k := range keys {
			all = append(all, k)
		}

		if isLast, _ := data["isLast"].(bool); isLast {
			break
		}
		token, _ := data["nextPageToken"].(string)
		if token == "" || len(keys) == 0 {
			break
		}
		nextToken = token
	}
	return all, nil
}

func issueKeysFromJQLSearchPayload(data map[string]any) []string {
	raw := data["issues"]
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		return keysFromIssueSlice(v)
	case map[string]any:
		nodes, ok := v["nodes"].([]any)
		if !ok {
			return nil
		}
		return keysFromIssueSlice(nodes)
	default:
		return nil
	}
}

func keysFromIssueSlice(items []any) []string {
	var out []string
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		k, _ := m["key"].(string)
		if strings.TrimSpace(k) != "" {
			out = append(out, strings.TrimSpace(k))
		}
	}
	return out
}

// IssueSummaryAssignee is a minimal issue view for dig (duplicate) workflows.
type IssueSummaryAssignee struct {
	Summary   string
	AccountID string // empty when unassigned
}

// GetIssueSummaryAssignee fetches summary and assignee account id for an issue key.
func (c *Client) GetIssueSummaryAssignee(ctx context.Context, key string) (*IssueSummaryAssignee, error) {
	escaped := url.PathEscape(strings.TrimSpace(key))
	u := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=summary,assignee", c.baseURL, escaped)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira API returned %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var raw struct {
		Fields struct {
			Summary  string `json:"summary"`
			Assignee *struct {
				AccountID string `json:"accountId"`
			} `json:"assignee"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	out := &IssueSummaryAssignee{Summary: strings.TrimSpace(raw.Fields.Summary)}
	if raw.Fields.Assignee != nil {
		out.AccountID = strings.TrimSpace(raw.Fields.Assignee.AccountID)
	}
	return out, nil
}

// CreateIssue creates an issue from a fields map (Jira REST "fields" object) and returns the new key.
func (c *Client) CreateIssue(ctx context.Context, fields map[string]any) (string, error) {
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return "", fmt.Errorf("marshal create body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/api/3/issue", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build create request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Content-Type", contentTypeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute create request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read create response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("jira create issue returned %d: %s", resp.StatusCode, truncate(string(respBody), 800))
	}

	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("parse create response: %w", err)
	}
	if strings.TrimSpace(created.Key) == "" {
		return "", fmt.Errorf("create issue returned no key")
	}
	return created.Key, nil
}

// CreateIssueLink creates a directed issue link between two keys (outward → inward per Jira semantics).
func (c *Client) CreateIssueLink(ctx context.Context, linkTypeName, outwardKey, inwardKey string) error {
	payload := map[string]any{
		"type":         map[string]string{"name": linkTypeName},
		"outwardIssue": map[string]string{"key": outwardKey},
		"inwardIssue":  map[string]string{"key": inwardKey},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal link body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/api/3/issueLink", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build link request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Content-Type", contentTypeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute link request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read link response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("jira issueLink returned %d: %s", resp.StatusCode, truncate(string(respBody), 800))
	}
	return nil
}
