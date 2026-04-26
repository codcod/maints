package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetMyselfAccountID returns the Jira accountId for the user authenticated with this client
// (GET /rest/api/3/myself). Used to compare issue assignee to the current user.
func (c *Client) GetMyselfAccountID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return "", fmt.Errorf("build myself request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jira myself %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	var me struct {
		AccountID string `json:"accountId"`
	}
	if err := json.Unmarshal(body, &me); err != nil {
		return "", fmt.Errorf("parse myself: %w", err)
	}
	s := strings.TrimSpace(me.AccountID)
	if s == "" {
		return "", fmt.Errorf("jira myself: empty accountId")
	}
	return s, nil
}
