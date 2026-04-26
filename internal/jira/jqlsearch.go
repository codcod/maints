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

// IssueJQL is one issue from POST /rest/api/3/search/jql with a requested field set.
type IssueJQL struct {
	Key    string
	Fields map[string]any
}

// JQLSearchWithFields runs a JQL search and returns all matching issues, paginated with nextPageToken.
// fieldNames are Jira field names (e.g. summary, status, duedate, issuelinks).
func (c *Client) JQLSearchWithFields(ctx context.Context, jql string, fieldNames []string) ([]IssueJQL, error) {
	if len(jql) == 0 {
		return nil, fmt.Errorf("jql is empty")
	}
	const pageSize = 50
	var all []IssueJQL
	var nextToken string
	for {
		payload := map[string]any{
			"jql":        jql,
			"maxResults": pageSize,
			"fields":     fieldNames,
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
			return nil, fmt.Errorf("execute search: %w", err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read search body: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jira search/jql returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
		}
		var data map[string]any
		if err := json.Unmarshal(respBody, &data); err != nil {
			return nil, fmt.Errorf("parse search: %w", err)
		}
		issues, err := issuesArrayFromJQL(data)
		if err != nil {
			return nil, err
		}
		for _, m := range issues {
			if k, _ := m["key"].(string); k != "" {
				fields, _ := m["fields"].(map[string]any)
				if fields == nil {
					fields = map[string]any{}
				}
				all = append(all, IssueJQL{Key: k, Fields: fields})
			}
		}
		if isLast, _ := data["isLast"].(bool); isLast {
			break
		}
		next := ""
		if t, _ := data["nextPageToken"].(string); t != "" {
			next = t
		}
		if next == "" || len(issues) == 0 {
			break
		}
		nextToken = next
	}
	return all, nil
}

// issuesArrayFromJQL returns the "issues" slice from a search response (array or { nodes: [...] }).
func issuesArrayFromJQL(data map[string]any) ([]map[string]any, error) {
	raw, ok := data["issues"]
	if !ok {
		return nil, nil
	}
	switch v := raw.(type) {
	case []any:
		return mapsFromIssueSlice(v)
	case map[string]any:
		nodes, _ := v["nodes"].([]any)
		return mapsFromIssueSlice(nodes)
	default:
		return nil, fmt.Errorf("search issues: unexpected type %T", v)
	}
}

func mapsFromIssueSlice(items []any) ([]map[string]any, error) {
	var out []map[string]any
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// GetIssueFields fetches a subset of fields for one issue (GET /rest/api/3/issue).
// JQL search often omits or strips issuelinks; use this to load them reliably.
func (c *Client) GetIssueFields(ctx context.Context, key string, fieldIDs []string) (map[string]any, error) {
	if len(fieldIDs) == 0 {
		return nil, fmt.Errorf("jira: GetIssueFields: no fields")
	}
	esc := url.PathEscape(strings.TrimSpace(key))
	f := strings.Join(fieldIDs, ",")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/rest/api/3/issue/"+esc+"?fields="+url.QueryEscape(f), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: %d %s", resp.StatusCode, truncate(string(body), 400))
	}
	var raw struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}
	if raw.Fields == nil {
		return map[string]any{}, nil
	}
	return raw.Fields, nil
}

// AssigneeString returns a display label for Jira's assignee field (object, null, or missing).
// Prefers displayName, then name, then emailAddress (when display is hidden by profile settings).
func AssigneeString(v any) string {
	if v == nil {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return ""
	}
	for _, k := range []string{"displayName", "name", "emailAddress"} {
		if s, _ := m[k].(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// FixVersionNamesString returns fix version names from Jira's `fixVersions` field
// value (array of objects with `name`), joined with `", "`. Returns "" if nil or empty.
func FixVersionNamesString(v any) string {
	arr, _ := v.([]any)
	if len(arr) == 0 {
		return ""
	}
	var names []string
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		n, _ := m["name"].(string)
		if t := strings.TrimSpace(n); t != "" {
			names = append(names, t)
		}
	}
	return strings.Join(names, ", ")
}

// GetIssueDashFields fetches status, assignee, summary, priority, and fixVersions for one issue.
func (c *Client) GetIssueDashFields(ctx context.Context, key string) (status, assignee, summary, priority, fixVersions string, err error) {
	escaped := url.PathEscape(strings.TrimSpace(key))
	u := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=status,assignee,summary,priority,fixVersions", c.baseURL, escaped)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", "", "", "", err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", "", "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", "", fmt.Errorf("jira: %d %s", resp.StatusCode, truncate(string(body), 200))
	}
	var raw struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", "", "", "", "", err
	}
	if raw.Fields == nil {
		return "", "", "", "", "", nil
	}
	if st, ok := raw.Fields["status"].(map[string]any); ok {
		if n, _ := st["name"].(string); n != "" {
			status = strings.TrimSpace(n)
		}
	}
	assignee = AssigneeString(raw.Fields["assignee"])
	if s, _ := raw.Fields["summary"].(string); s != "" {
		summary = strings.TrimSpace(s)
	}
	if pr, ok := raw.Fields["priority"].(map[string]any); ok {
		if n, _ := pr["name"].(string); n != "" {
			priority = strings.TrimSpace(n)
		}
	}
	fixVersions = FixVersionNamesString(raw.Fields["fixVersions"])
	return status, assignee, summary, priority, fixVersions, nil
}
