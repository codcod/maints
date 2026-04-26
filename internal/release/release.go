package release

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/codcod/maints-triage/internal/dig"
	"github.com/codcod/maints-triage/internal/jira"
)

// Options configures a release run (DIG issues in fix version, linked MAINTs).
type Options struct {
	// FixVersion is the Jira fix version name (e.g. DS 2025.09.2).
	FixVersion   string
	DigProject   string
	MaintProject string
	// LinkType matches maints dig / maints dash (JIRA_LINK_TYPE or "Solved by" pair with DIG/MAINT).
	LinkType string
	// Supervisor, when true, allows closing MAINTs regardless of assignee. Default: only
	// close when the MAINT is assigned to the authenticated user (Jira "current user").
	Supervisor bool
}

// Run processes DIGs with the given fix version: terminal DIGs are linked to MAINTs, then
// each unique MAINT is either transitioned to Done with a closing comment, or an "open" comment.
func Run(ctx context.Context, c *jira.Client, o Options, out, errW io.Writer) error {
	ver := strings.TrimSpace(o.FixVersion)
	if ver == "" {
		return fmt.Errorf("fix version is required")
	}
	digProj := strings.ToUpper(strings.TrimSpace(o.DigProject))
	if digProj == "" {
		digProj = "DIG"
	}
	maintProj := strings.ToUpper(strings.TrimSpace(o.MaintProject))
	if maintProj == "" {
		maintProj = "MAINT"
	}
	lt := strings.TrimSpace(o.LinkType)
	if lt == "" {
		lt = dig.DefaultLinkType()
	}

	if err := c.EnsureProjectFixVersionReleased(ctx, digProj, ver); err != nil {
		return err
	}

	selfID, err := c.GetMyselfAccountID(ctx)
	if err != nil {
		return err
	}

	jql := defaultReleaseJQL(digProj, ver)
	digKeys, err := c.SearchIssueKeysPOST(ctx, jql)
	if err != nil {
		return err
	}
	if len(digKeys) == 0 {
		_, _ = fmt.Fprintln(errW, "No issues matched the query.")
		return nil
	}
	_, _ = fmt.Fprintf(out, "release: %d DIG issue(s) in scope: %s\n", len(digKeys), jql)

	maintToTouch := make(map[string]struct{})
	for _, raw := range digKeys {
		dk := strings.ToUpper(strings.TrimSpace(raw))
		if dk == "" {
			continue
		}
		fields, err := c.GetIssueFieldsMap(ctx, dk, "status,issuelinks")
		if err != nil {
			_, _ = fmt.Fprintf(errW, "warn: %s: load fields: %v; skip\n", dk, err)
			continue
		}
		st := statusName(fields)
		if !isDoneOrClosed(st) {
			_, _ = fmt.Fprintf(errW, "warn: %s: status is %q (not Done/Closed); skip for MAINT follow-up\n", dk, st)
			continue
		}
		for _, mk := range findMaintsForDig(linksSlice(fields), dk, lt, maintProj) {
			maintToTouch[mk] = struct{}{}
		}
	}
	if len(maintToTouch) == 0 {
		_, _ = fmt.Fprintln(errW, "No linked MAINTs to process (from Done/Closed DIGs in this fix version).")
		return nil
	}

	keys := make([]string, 0, len(maintToTouch))
	for m := range maintToTouch {
		keys = append(keys, m)
	}
	sort.Strings(keys)

	for _, maintKey := range keys {
		if err := processMaint(ctx, c, selfID, o.Supervisor, maintKey, ver, digProj, lt, out, errW); err != nil {
			return err
		}
	}
	return nil
}

func processMaint(ctx context.Context, c *jira.Client, myAccountID string, supervisor bool, maintKey, fixVersion, digProj, linkType string, out, errW io.Writer) error {
	fields, err := c.GetIssueFieldsMap(ctx, maintKey, "issuelinks,status,assignee")
	if err != nil {
		_, _ = fmt.Fprintf(errW, "fail: %s: %v\n", maintKey, err)
		return err
	}
	if st := statusName(fields); isDoneOrClosed(st) {
		_, _ = fmt.Fprintf(out, "info: %s already %q; skip transition and comment\n", maintKey, st)
		return nil
	}

	solved := solvedByDigKeys(linksSlice(fields), maintKey, linkType, digProj)
	if len(solved) == 0 {
		_, _ = fmt.Fprintf(errW, "warn: %s: no %q link to %s-…; nothing to do\n", maintKey, linkType, digProj)
		return nil
	}

	terminal, err := allDigsDoneOrClosed(ctx, c, solved)
	if err != nil {
		_, _ = fmt.Fprintf(errW, "fail: %s: %v\n", maintKey, err)
		return err
	}
	closingMsg := plainCommentADF("All patches have been released now, closing.")
	keepOpenMsg := plainCommentADF("Patch " + fixVersion + " has been released, keeping MAINT open until all patches are released.")

	if terminal {
		if !supervisor {
			aid := assigneeAccountID(fields)
			if aid == "" || !strings.EqualFold(aid, myAccountID) {
				_, _ = fmt.Fprintf(out, "info: %s: skip closing — MAINT is not assigned to you (use --supervisor to close any assignee)\n", maintKey)
				return nil
			}
		}
		if err := c.TransitionToStatusName(ctx, maintKey, "Done", "Done"); err != nil {
			_, _ = fmt.Fprintf(errW, "fail: %s: transition to Done: %v\n", maintKey, err)
			return err
		}
		_, _ = fmt.Fprintf(out, "ok: %s transitioned to Done\n", maintKey)
		if err := c.AddIssueComment(ctx, maintKey, closingMsg); err != nil {
			_, _ = fmt.Fprintf(errW, "fail: %s: comment: %v\n", maintKey, err)
			return err
		}
		_, _ = fmt.Fprintf(out, "ok: %s commented\n", maintKey)
		return nil
	}

	_, _ = fmt.Fprintf(out, "ok: %s: some DIGs still not Done/Closed; comment only\n", maintKey)
	if err := c.AddIssueComment(ctx, maintKey, keepOpenMsg); err != nil {
		_, _ = fmt.Fprintf(errW, "fail: %s: comment: %v\n", maintKey, err)
		return err
	}
	_, _ = fmt.Fprintf(out, "ok: %s commented\n", maintKey)
	return nil
}

func assigneeAccountID(fields map[string]any) string {
	if fields == nil {
		return ""
	}
	m, _ := fields["assignee"].(map[string]any)
	if m == nil {
		return ""
	}
	if id, _ := m["accountId"].(string); id != "" {
		return strings.TrimSpace(id)
	}
	return ""
}

func allDigsDoneOrClosed(ctx context.Context, c *jira.Client, digKeys []string) (bool, error) {
	for _, d := range digKeys {
		st, _, _, _, _, err := c.GetIssueDashFields(ctx, d)
		if err != nil {
			return false, err
		}
		if !isDoneOrClosed(st) {
			return false, nil
		}
	}
	return true, nil
}

// defaultReleaseJQL matches DIG issues carrying this fix version name.
func defaultReleaseJQL(digProject, version string) string {
	// Jira JQL: fixVersion field by name (see maints fixversion and JQL docs).
	esc := jqlStringLiteral(version)
	return "project = " + strings.ToUpper(digProject) + " AND fixVersion = " + esc
}

func jqlStringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func isDoneOrClosed(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return strings.EqualFold(s, "Done") || strings.EqualFold(s, "Closed")
}

func statusName(fields map[string]any) string {
	if fields == nil {
		return ""
	}
	st, _ := fields["status"].(map[string]any)
	if st == nil {
		return ""
	}
	if n, _ := st["name"].(string); n != "" {
		return strings.TrimSpace(n)
	}
	return ""
}

func linksSlice(fields map[string]any) []any {
	if fields == nil {
		return nil
	}
	links, _ := fields["issuelinks"].([]any)
	return links
}

// findMaintsForDig returns MAINT issue keys on the other end of a link of type
// `linkType` (same as maints dig) from a DIG in fix version scope.
func findMaintsForDig(issuelinks []any, digKey, wantLink, maintProject string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, li := range issuelinks {
		link, _ := li.(map[string]any)
		if link == nil {
			continue
		}
		lt, _ := link["type"].(map[string]any)
		if !linkTypeMatch(wantLink, lt) {
			continue
		}
		ik := issueKeyFromLinkSide(link, "inwardIssue")
		oke := issueKeyFromLinkSide(link, "outwardIssue")
		if ik == "" && oke != "" {
			ik = digKey
		}
		if oke == "" && ik != "" {
			oke = digKey
		}
		if strings.EqualFold(ik, digKey) {
			if isProjKey(oke, maintProject) {
				if _, d := seen[oke]; !d {
					seen[oke] = struct{}{}
					out = append(out, oke)
				}
			}
			continue
		}
		if strings.EqualFold(oke, digKey) {
			if isProjKey(ik, maintProject) {
				if _, d := seen[ik]; !d {
					seen[ik] = struct{}{}
					out = append(out, ik)
				}
			}
		}
	}
	return out
}

func isProjKey(key, project string) bool {
	k := strings.ToUpper(strings.TrimSpace(key))
	p := strings.ToUpper(project) + "-"
	return k != "" && strings.HasPrefix(k, p)
}

// linkTypeMatch mirrors maints dash and fixversion (Jira name / inward / outward, substring).
func linkTypeMatch(want string, typeMap map[string]any) bool {
	if want == "" || typeMap == nil {
		return false
	}
	want = strings.TrimSpace(want)
	lowWant := strings.ToLower(want)
	for _, f := range []string{"name", "inward", "outward"} {
		s := asString(typeMap[f])
		if s == "" {
			continue
		}
		if strings.EqualFold(s, want) {
			return true
		}
		if strings.Contains(strings.ToLower(s), lowWant) {
			return true
		}
	}
	return false
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func issueKeyFromLinkSide(link map[string]any, which string) string {
	m, _ := link[which].(map[string]any)
	if m == nil {
		return ""
	}
	if k, _ := m["key"].(string); k != "" {
		return strings.TrimSpace(k)
	}
	return ""
}

// solvedByDigKeys returns DIG keys on MAINT links of type `linkType` (dash/dig behavior).
func solvedByDigKeys(issuelinks []any, maintKey, wantLinkType, digProject string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, l := range issuelinks {
		m, _ := l.(map[string]any)
		if m == nil {
			continue
		}
		lt, _ := m["type"].(map[string]any)
		if !linkTypeMatch(wantLinkType, lt) {
			continue
		}
		digKey, ok := pickDIGEnd(maintKey, m, digProject)
		if !ok || digKey == "" {
			continue
		}
		if _, dup := seen[digKey]; dup {
			continue
		}
		seen[digKey] = struct{}{}
		out = append(out, digKey)
	}
	return out
}

// pickDIGEnd is copied from maints dash (Solved by → DIG on one end).
func pickDIGEnd(maintKey string, link map[string]any, digProject string) (digKey string, ok bool) {
	inW, inOK := link["inwardIssue"].(map[string]any)
	outW, outOK := link["outwardIssue"].(map[string]any)
	if !inOK || inW == nil {
		if outOK && outW != nil {
			oke := issueKeyFromLinkSide(link, "outwardIssue")
			if isDigKey(oke, digProject) && oke != "" {
				return oke, true
			}
		}
	}
	if !outOK || outW == nil {
		if inOK && inW != nil {
			ik := issueKeyFromLinkSide(link, "inwardIssue")
			if isDigKey(ik, digProject) && ik != "" {
				return ik, true
			}
		}
	}
	ik := issueKeyFromLinkSide(link, "inwardIssue")
	oke := issueKeyFromLinkSide(link, "outwardIssue")
	if oke != "" && strings.EqualFold(ik, maintKey) && isDigKey(oke, digProject) {
		return oke, true
	}
	if ik != "" && strings.EqualFold(oke, maintKey) && isDigKey(ik, digProject) {
		return ik, true
	}
	return "", false
}

func isDigKey(key, digProject string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	p := strings.ToUpper(digProject) + "-"
	return strings.HasPrefix(upper, p)
}

func plainCommentADF(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": text},
				},
			},
		},
	}
}
