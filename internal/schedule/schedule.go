package schedule

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/codcod/maints-triage/internal/dig"
	"github.com/codcod/maints-triage/internal/jira"
)

const patchReleasesWikiURL = "https://backbase.atlassian.net/wiki/x/XAC5CAE"

// Options are DIG keys or JQL, versions, and which link to follow to MAINT.
type Options struct {
	Keys         []string
	JQL          string
	Versions     []string
	Remove       bool
	LinkType     string
	DigProject   string
	MaintProject string
}

// Run sets or removes fix versions on DIG issues; on each successful Jira update it comments on
// linked MAINTs (link type, e.g. Solved by).
func Run(ctx context.Context, client *jira.Client, o Options, out, errOut io.Writer) error {
	vers := uniqueTrimmed(o.Versions)
	if len(vers) == 0 {
		return fmt.Errorf("at least one --version is required")
	}
	lt := strings.TrimSpace(o.LinkType)
	if lt == "" {
		lt = dig.DefaultLinkType()
	}
	digProj := strings.ToUpper(strings.TrimSpace(o.DigProject))
	if digProj == "" {
		digProj = "DIG"
	}
	maintProj := strings.ToUpper(strings.TrimSpace(o.MaintProject))
	if maintProj == "" {
		maintProj = "MAINT"
	}
	digPrefix := digProj + "-"

	var keys []string
	if strings.TrimSpace(o.JQL) != "" {
		var err error
		keys, err = client.SearchIssueKeysPOST(ctx, strings.TrimSpace(o.JQL))
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			_, _ = fmt.Fprintln(errOut, "No issues matched the query.")
			return nil
		}
	} else {
		keys = append(keys, o.Keys...)
	}

	var failures int
	for _, raw := range keys {
		digKey := strings.ToUpper(strings.TrimSpace(raw))
		if digKey == "" {
			_, _ = fmt.Fprintf(errOut, "FAIL %q: empty key\n", raw)
			failures++
			continue
		}
		if !strings.HasPrefix(digKey, digPrefix) {
			_, _ = fmt.Fprintf(errOut, "FAIL %s: not in project %q (use --dig-project to match query)\n", digKey, digProj)
			failures++
			continue
		}

		err := runOne(ctx, client, o.Remove, digKey, vers, lt, maintProj, errOut)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "FAIL %s\n%s\n", digKey, err)
			failures++
			continue
		}
		_, _ = fmt.Fprintf(out, "OK %s\n", digKey)
	}
	if failures > 0 {
		return fmt.Errorf("%d of %d issue(s) failed", failures, len(keys))
	}
	return nil
}

func runOne(ctx context.Context, client *jira.Client, remove bool, digKey string, versions []string,
	wantLink, maintProject string, errOut io.Writer) error {
	fields, err := client.GetIssueFieldsMap(ctx, digKey, "fixVersions,issuelinks")
	if err != nil {
		return err
	}
	links, _ := fields["issuelinks"].([]any)
	if fields == nil {
		fields = map[string]any{}
	}

	currentObjs, currentNames := parseFixVersions(fields["fixVersions"])
	var newObjs []map[string]any
	var toComment []string

	if remove {
		rem := foldSet(versions)
		removedNameByFold := make(map[string]string) // lower -> Jira display name
		for _, n := range currentNames {
			if rem[strings.ToLower(n)] {
				removedNameByFold[strings.ToLower(n)] = n
			}
		}
		if len(removedNameByFold) == 0 {
			return fmt.Errorf("none of the requested fix versions are set on the issue: %q", strings.Join(versions, `", "`))
		}
		seenCmt := make(map[string]struct{})
		for _, w := range versions {
			lf := strings.ToLower(strings.TrimSpace(w))
			if display, ok := removedNameByFold[lf]; ok {
				if _, d := seenCmt[display]; d {
					continue
				}
				seenCmt[display] = struct{}{}
				toComment = append(toComment, display)
			}
		}
		for _, o := range currentObjs {
			n, _ := o["name"].(string)
			if n == "" {
				continue
			}
			if !rem[strings.ToLower(strings.TrimSpace(n))] {
				if u := fixVersionUpdateValue(o); u != nil {
					newObjs = append(newObjs, u)
				}
			}
		}
	} else {
		existing := foldSet(currentNames)
		var added []string
		for _, w := range versions {
			if !existing[strings.ToLower(strings.TrimSpace(w))] {
				added = append(added, w)
			}
		}
		if len(added) == 0 {
			return nil
		}
		for _, o := range currentObjs {
			if u := fixVersionUpdateValue(o); u != nil {
				newObjs = append(newObjs, u)
			}
		}
		for _, w := range added {
			newObjs = append(newObjs, map[string]any{"name": w})
		}
		toComment = added
	}
	if err := client.UpdateIssue(ctx, digKey, map[string]any{"fixVersions": newObjs}); err != nil {
		return err
	}
	maintKeys := findLinkedMaints(links, digKey, wantLink, maintProject)
	if len(maintKeys) == 0 {
		_, _ = fmt.Fprintf(errOut, "warn: no %s link to %s; skipped MAINT comment(s)\n", wantLink, maintProject)
		return nil
	}
	if remove {
		for _, v := range toComment {
			for _, m := range maintKeys {
				adf := commentADFForRemoved(m, v, patchReleasesWikiURL)
				if err := client.AddIssueComment(ctx, m, adf); err != nil {
					return fmt.Errorf("comment on %s: %w", m, err)
				}
			}
		}
		return nil
	}
	for _, v := range toComment {
		for _, m := range maintKeys {
			adf := commentADFForAdded(m, v, patchReleasesWikiURL)
			if err := client.AddIssueComment(ctx, m, adf); err != nil {
				return fmt.Errorf("comment on %s: %w", m, err)
			}
		}
	}
	return nil
}

func uniqueTrimmed(ss []string) (out []string) {
	seen := map[string]struct{}{}
	for _, s := range ss {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		k := strings.ToLower(t)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, t)
	}
	return out
}

func parseFixVersions(v any) (objs []map[string]any, names []string) {
	arr, _ := v.([]any)
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		objs = append(objs, m)
		if n, _ := m["name"].(string); strings.TrimSpace(n) != "" {
			names = append(names, strings.TrimSpace(n))
		}
	}
	return objs, names
}

func fixVersionUpdateValue(m map[string]any) map[string]any {
	if id, _ := m["id"].(string); strings.TrimSpace(id) != "" {
		return map[string]any{"id": strings.TrimSpace(id)}
	}
	if n, _ := m["name"].(string); strings.TrimSpace(n) != "" {
		return map[string]any{"name": strings.TrimSpace(n)}
	}
	return nil
}

func foldSet(ss []string) map[string]bool {
	m := make(map[string]bool)
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			m[strings.ToLower(t)] = true
		}
	}
	return m
}

func findLinkedMaints(issuelinks []any, digKey, wantLink, maintProject string) []string {
	seen := map[string]struct{}{}
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
			other := oke
			if isProjKey(other, maintProject) {
				if _, du := seen[other]; !du {
					seen[other] = struct{}{}
					out = append(out, other)
				}
			}
			continue
		}
		if strings.EqualFold(oke, digKey) {
			other := ik
			if isProjKey(other, maintProject) {
				if _, du := seen[other]; !du {
					seen[other] = struct{}{}
					out = append(out, other)
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

// linkTypeMatch mirrors the dash table logic: match Jira's link type to the configured name
// (e.g. "Solved by" vs inward "is solved by" / name "Solves").
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
	k, _ := m["key"].(string)
	return strings.TrimSpace(k)
}

// commentADFForAdded and commentADFForRemoved build the text from the spec (Jira markdown-style link
// in prose → ADF with one link to Patch Releases).
func commentADFForAdded(_, version, url string) map[string]any {
	// "Fix for this MAINT has been added to the scope of [version] patch.  See details of this patch in [Patch Releases](url) page."
	lead := "Fix for this MAINT has been added to the scope of " + version + " patch.  See details of this patch in "
	rest := " page."
	return paraWithLink(lead, "Patch Releases", url, rest)
}

func commentADFForRemoved(_, version, u string) map[string]any {
	lead := "Fix for this MAINT has been removed from the scope of " + version + " patch.  To see what is the scope of this patch, go to "
	rest := " page."
	return paraWithLink(lead, "Patch Releases", u, rest)
}

func paraWithLink(before, linkText, linkURL, after string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": before},
					map[string]any{
						"type": "text",
						"text": linkText,
						"marks": []any{map[string]any{
							"type": "link",
							"attrs": map[string]string{
								"href": linkURL,
							},
						}},
					},
					map[string]any{"type": "text", "text": after},
				},
			},
		},
	}
}
