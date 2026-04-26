package dig

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/codcod/maints-triage/internal/jira"
)

// Options configures a dig run (duplicate source issues into the DIG project).
type Options struct {
	Keys         []string
	JQL          string
	DigProject   string
	IssueType    string
	LinkType     string
	LinkSwapEnds bool
	TeamField    string // Jira custom field id (e.g. customfield_14700); required, from JIRA_TEAM_FIELD
	TeamID       string // Atlassian team UUID; required, from JIRA_TEAM_ID
}

// DefaultIssueType returns $JIRA_DIG_ISSUE_TYPE or "Bug".
func DefaultIssueType() string {
	v := strings.TrimSpace(os.Getenv("JIRA_DIG_ISSUE_TYPE"))
	if v != "" {
		return v
	}
	return "Bug"
}

// DefaultLinkType returns $JIRA_LINK_TYPE, else $JIRA_SOLVES_LINK_TYPE, else "Solved by".
func DefaultLinkType() string {
	if v := strings.TrimSpace(os.Getenv("JIRA_LINK_TYPE")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("JIRA_SOLVES_LINK_TYPE")); v != "" {
		return v
	}
	return "Solved by"
}

// duplicateDescriptionADF returns Jira ADF for the DIG issue description. The source
// issue key is hyperlinked to sourceBrowseURL (typically …/browse/MAINT-123).
func duplicateDescriptionADF(sourceKey, sourceBrowseURL string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": "This is a duplicate of "},
					map[string]any{
						"type": "text",
						"text": sourceKey,
						"marks": []any{
							map[string]any{
								"type": "link",
								"attrs": map[string]string{
									"href": sourceBrowseURL,
								},
							},
						},
					},
					map[string]any{
						"type": "text",
						"text": " for internal processing. The source of truth is the MAINT ticket.",
					},
				},
			},
		},
	}
}

// Run duplicates each source issue into DigProject, sets team, optional assignee, and creates an issue link.
func Run(ctx context.Context, client *jira.Client, opts Options, out, errOut io.Writer) error {
	if strings.TrimSpace(opts.TeamField) == "" {
		return fmt.Errorf("missing required environment variable: JIRA_TEAM_FIELD")
	}
	if strings.TrimSpace(opts.TeamID) == "" {
		return fmt.Errorf("missing required environment variable: JIRA_TEAM_ID")
	}
	if strings.TrimSpace(opts.DigProject) == "" {
		return fmt.Errorf("dig project key is empty")
	}

	var keys []string
	if strings.TrimSpace(opts.JQL) != "" {
		var err error
		keys, err = client.SearchIssueKeysPOST(ctx, strings.TrimSpace(opts.JQL))
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			_, _ = fmt.Fprintln(errOut, "No issues matched the query.")
			return nil
		}
	} else {
		keys = append(keys, opts.Keys...)
	}

	var failures int
	for _, raw := range keys {
		k := strings.TrimSpace(strings.ToUpper(raw))
		if k == "" {
			_, _ = fmt.Fprintf(errOut, "FAIL %q\nempty key\n", raw)
			failures++
			continue
		}

		info, err := client.GetIssueSummaryAssignee(ctx, k)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "FAIL %s\n%s\n", k, err)
			failures++
			continue
		}
		if info.Summary == "" {
			_, _ = fmt.Fprintf(errOut, "FAIL %s\n%s: could not read summary; refuse to create a duplicate with empty title\n", k, k)
			failures++
			continue
		}

		browse := fmt.Sprintf("%s/browse/%s", client.BaseURL(), k)
		fields := map[string]any{
			"project":      map[string]string{"key": opts.DigProject},
			"issuetype":    map[string]string{"name": opts.IssueType},
			"summary":      info.Summary,
			"description":  duplicateDescriptionADF(k, browse),
			opts.TeamField: strings.TrimSpace(opts.TeamID),
		}
		if info.AccountID != "" {
			fields["assignee"] = map[string]string{"id": info.AccountID}
		}

		newKey, err := client.CreateIssue(ctx, fields)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "FAIL %s\n%s\n", k, err)
			failures++
			continue
		}

		outwardKey, inwardKey := newKey, k
		if opts.LinkSwapEnds {
			outwardKey, inwardKey = k, newKey
		}
		if err := client.CreateIssueLink(ctx, opts.LinkType, outwardKey, inwardKey); err != nil {
			_, _ = fmt.Fprintf(errOut, "FAIL %s\n%s: created %s but issue link failed: %s\n", k, k, newKey, err)
			failures++
			continue
		}

		_, _ = fmt.Fprintf(out, "OK %s -> %s\n", k, newKey)
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d source issue(s) failed", failures, len(keys))
	}
	return nil
}
