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

// Transition is one workflow transition as returned by GET /issue/{key}/transitions.
type Transition struct {
	ID   string
	Name string
	// ToStatusName is the name of the destination Jira status (e.g. "Done").
	ToStatusName string
	// HasResolution is true if the expand metadata says resolution can be set on this transition.
	HasResolution bool
	Raw           map[string]any
}

// ListTransitions returns available transitions for an issue
// (GET /rest/api/3/issue/{key}/transitions?expand=transitions.fields).
func (c *Client) ListTransitions(ctx context.Context, issueKey string) ([]Transition, error) {
	esc := url.PathEscape(strings.TrimSpace(issueKey))
	u := c.baseURL + "/rest/api/3/issue/" + esc + "/transitions?expand=transitions.fields"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build transitions request: %w", err)
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
		return nil, fmt.Errorf("jira get transitions %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	var data struct {
		Transitions []map[string]any `json:"transitions"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse transitions: %w", err)
	}
	var out []Transition
	for _, m := range data.Transitions {
		var id string
		switch v := m["id"].(type) {
		case string:
			id = v
		case float64:
			id = fmt.Sprintf("%.0f", v)
		case json.Number:
			id = v.String()
		}
		name, _ := m["name"].(string)
		t := Transition{ID: id, Name: name, Raw: m}
		if to, _ := m["to"].(map[string]any); to != nil {
			if n, _ := to["name"].(string); n != "" {
				t.ToStatusName = n
			}
		}
		if f, ok := m["fields"].(map[string]any); ok {
			if _, res := f["resolution"]; res {
				t.HasResolution = true
			}
		}
		if t.ID != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// PostTransition performs POST /rest/api/3/issue/{key}/transitions.
// If fields is non-empty it is set as the top-level "fields" object in the request body.
func (c *Client) PostTransition(ctx context.Context, issueKey, transitionID string, fields map[string]any) error {
	esc := url.PathEscape(strings.TrimSpace(issueKey))
	u := c.baseURL + "/rest/api/3/issue/" + esc + "/transitions"
	body := map[string]any{
		"transition": map[string]any{
			"id": transitionID,
		},
	}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build transition request: %w", err)
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
		return fmt.Errorf("jira post transition %d: %s", resp.StatusCode, truncate(string(rbody), 800))
	}
	return nil
}

// TransitionToStatusName finds a transition whose ToStatusName equals wantStatus (case-insensitive)
// and performs it. If resolutionName is set, the first attempt includes that resolution; on failure
// a second attempt without extra fields (for workflows with no resolution), then third with
// resolution "Fixed" as a common fallback, is tried.
func (c *Client) TransitionToStatusName(ctx context.Context, issueKey, wantStatus, resolutionName string) error {
	tlist, err := c.ListTransitions(ctx, issueKey)
	if err != nil {
		return err
	}
	var pick *Transition
	for i := range tlist {
		if strings.EqualFold(strings.TrimSpace(tlist[i].ToStatusName), wantStatus) {
			pick = &tlist[i]
			break
		}
	}
	if pick == nil {
		return fmt.Errorf("no available transition to status %q for %s", wantStatus, issueKey)
	}
	tid := pick.ID

	var order []map[string]any
	if s := strings.TrimSpace(resolutionName); s != "" {
		order = append(order, map[string]any{"resolution": map[string]any{"name": s}})
	}
	order = append(order, nil)
	// Jira often accepts "Fixed" or "Done" for resolution; try "Fixed" if a named resolution was requested
	if s := strings.TrimSpace(resolutionName); s != "" {
		if !strings.EqualFold(s, "Fixed") {
			order = append(order, map[string]any{"resolution": map[string]any{"name": "Fixed"}})
		}
	}

	var last error
	for _, f := range order {
		if err := c.PostTransition(ctx, issueKey, tid, f); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}
