package dig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDuplicateDescriptionADF(t *testing.T) {
	browse := "https://example.atlassian.net/browse/MAINT-42"
	adf := duplicateDescriptionADF("MAINT-42", browse)
	b, err := json.Marshal(adf)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "doc" {
		t.Fatalf("type = %v", m["type"])
	}
	want := "This is a duplicate of MAINT-42 for internal processing. The source of truth is the MAINT ticket."
	if got := paragraphPlainText(adf); got != want {
		t.Fatalf("paragraph text = %q, want %q", got, want)
	}
	if got := linkHrefForIssueKey(adf, "MAINT-42"); got != browse {
		t.Fatalf("link href = %q, want %q", got, browse)
	}
}

func paragraphPlainText(adf map[string]any) string {
	content, _ := adf["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	para, _ := content[0].(map[string]any)
	inner, _ := para["content"].([]any)
	var parts []string
	for _, node := range inner {
		m, ok := node.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["text"].(string); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "")
}

func linkHrefForIssueKey(adf map[string]any, key string) string {
	content, _ := adf["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	para, _ := content[0].(map[string]any)
	inner, _ := para["content"].([]any)
	for _, node := range inner {
		m, ok := node.(map[string]any)
		if !ok {
			continue
		}
		if m["text"] != key {
			continue
		}
		marks, _ := m["marks"].([]any)
		for _, mark := range marks {
			mm, ok := mark.(map[string]any)
			if !ok || mm["type"] != "link" {
				continue
			}
			if href := hrefFromAttrs(mm["attrs"]); href != "" {
				return href
			}
		}
	}
	return ""
}

func hrefFromAttrs(attrs any) string {
	switch a := attrs.(type) {
	case map[string]string:
		return a["href"]
	case map[string]any:
		if href, ok := a["href"].(string); ok {
			return href
		}
	}
	return ""
}

func TestDefaultIssueType(t *testing.T) {
	t.Setenv("JIRA_DIG_ISSUE_TYPE", "")
	if got := DefaultIssueType(); got != "Bug" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultIssueType_env(t *testing.T) {
	t.Setenv("JIRA_DIG_ISSUE_TYPE", "Story")
	if got := DefaultIssueType(); got != "Story" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultLinkType_precedence(t *testing.T) {
	t.Setenv("JIRA_LINK_TYPE", "Blocks")
	t.Setenv("JIRA_SOLVES_LINK_TYPE", "Solved by")
	if got := DefaultLinkType(); got != "Blocks" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultLinkType_legacy(t *testing.T) {
	t.Setenv("JIRA_LINK_TYPE", "")
	t.Setenv("JIRA_SOLVES_LINK_TYPE", "Duplicate")
	if got := DefaultLinkType(); got != "Duplicate" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultLinkType_fallback(t *testing.T) {
	t.Setenv("JIRA_LINK_TYPE", "")
	t.Setenv("JIRA_SOLVES_LINK_TYPE", "")
	if got := DefaultLinkType(); got != "Solved by" {
		t.Fatalf("got %q", got)
	}
}
