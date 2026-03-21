package triage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codcod/maints-triage/internal/jira"
)

func TestTriageHome(t *testing.T) {
	t.Run("TRIAGE_HOME used when set", func(t *testing.T) {
		t.Setenv("TRIAGE_HOME", "/custom/triage")
		got, err := triageHome()
		if err != nil {
			t.Fatalf("triageHome() error = %v", err)
		}
		if got != "/custom/triage" {
			t.Errorf("got %q, want %q", got, "/custom/triage")
		}
	})

	t.Run("falls back to XDG_CONFIG_HOME/triage", func(t *testing.T) {
		t.Setenv("TRIAGE_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
		got, err := triageHome()
		if err != nil {
			t.Fatalf("triageHome() error = %v", err)
		}
		want := filepath.Join("/xdg/config", "triage")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestResolveChecklist(t *testing.T) {
	t.Run("explicit path returned as-is", func(t *testing.T) {
		got, err := resolveChecklist("/some/explicit/checklist.md")
		if err != nil {
			t.Fatalf("resolveChecklist() error = %v", err)
		}
		if got != "/some/explicit/checklist.md" {
			t.Errorf("got %q, want %q", got, "/some/explicit/checklist.md")
		}
	})

	t.Run("TRIAGE_HOME used when set and file exists", func(t *testing.T) {
		tmp := t.TempDir()
		checklistPath := filepath.Join(tmp, "checklist.md")
		if err := os.WriteFile(checklistPath, []byte("# TRIAGE_HOME checklist"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TRIAGE_HOME", tmp)

		got, err := resolveChecklist("")
		if err != nil {
			t.Fatalf("resolveChecklist() error = %v", err)
		}
		if got != checklistPath {
			t.Errorf("got %q, want %q", got, checklistPath)
		}
	})

	t.Run("TRIAGE_HOME takes priority over XDG_CONFIG_HOME", func(t *testing.T) {
		tmp := t.TempDir()
		triageHomeDir := filepath.Join(tmp, "triage-home")
		if err := os.MkdirAll(triageHomeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		triageChecklist := filepath.Join(triageHomeDir, "checklist.md")
		if err := os.WriteFile(triageChecklist, []byte("# TRIAGE_HOME"), 0o644); err != nil {
			t.Fatal(err)
		}

		xdgDir := filepath.Join(tmp, "xdg", "triage")
		if err := os.MkdirAll(xdgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(xdgDir, "checklist.md"), []byte("# XDG"), 0o644); err != nil {
			t.Fatal(err)
		}

		t.Setenv("TRIAGE_HOME", triageHomeDir)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg"))

		got, err := resolveChecklist("")
		if err != nil {
			t.Fatalf("resolveChecklist() error = %v", err)
		}
		if got != triageChecklist {
			t.Errorf("got %q, want %q", got, triageChecklist)
		}
	})

	t.Run("XDG path used when file exists and TRIAGE_HOME is unset", func(t *testing.T) {
		tmp := t.TempDir()
		xdgDir := filepath.Join(tmp, "triage")
		if err := os.MkdirAll(xdgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		checklistPath := filepath.Join(xdgDir, "checklist.md")
		if err := os.WriteFile(checklistPath, []byte("# XDG checklist"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TRIAGE_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", tmp)

		got, err := resolveChecklist("")
		if err != nil {
			t.Fatalf("resolveChecklist() error = %v", err)
		}
		if got != checklistPath {
			t.Errorf("got %q, want %q", got, checklistPath)
		}
	})

	t.Run("falls back to ./checklist.md when no config file found", func(t *testing.T) {
		t.Setenv("TRIAGE_HOME", "")
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		got, err := resolveChecklist("")
		if err != nil {
			t.Fatalf("resolveChecklist() error = %v", err)
		}
		if got != "checklist.md" {
			t.Errorf("got %q, want %q", got, "checklist.md")
		}
	})
}

func TestLoadKBIndex(t *testing.T) {
	t.Run("returns nil when file is absent", func(t *testing.T) {
		t.Setenv("TRIAGE_HOME", t.TempDir())
		data, err := loadKBIndex()
		if err != nil {
			t.Fatalf("loadKBIndex() error = %v", err)
		}
		if data != nil {
			t.Errorf("expected nil when kb-index.md is absent, got %d bytes", len(data))
		}
	})

	t.Run("returns file content when present", func(t *testing.T) {
		tmp := t.TempDir()
		content := []byte("# KB Index\n## Section A\n")
		if err := os.WriteFile(filepath.Join(tmp, defaultKBIndexFile), content, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TRIAGE_HOME", tmp)

		data, err := loadKBIndex()
		if err != nil {
			t.Fatalf("loadKBIndex() error = %v", err)
		}
		if !bytes.Equal(data, content) {
			t.Errorf("loadKBIndex() = %q, want %q", data, content)
		}
	})
}

func TestBuildPrompt(t *testing.T) {
	key := "MAINT-42"
	template := `Read the file "issue-{{ISSUE_KEY}}.md" and evaluate it against each item in "checklist.md".
  ✅ Complete
  ❌ Missing
` + VerdictPass + ` or ` + VerdictFail + ` verdict.`
	prompt := strings.ReplaceAll(template, promptKeyPlaceholder, key)

	if !strings.Contains(prompt, "issue-MAINT-42.md") {
		t.Errorf("prompt should reference the issue file, got: %s", prompt)
	}
	if !strings.Contains(prompt, "checklist.md") {
		t.Errorf("prompt should reference checklist.md, got: %s", prompt)
	}
	if !strings.Contains(prompt, "✅") {
		t.Errorf("prompt should contain ✅ status marker")
	}
	if !strings.Contains(prompt, "❌") {
		t.Errorf("prompt should contain ❌ status marker")
	}
	if !strings.Contains(prompt, VerdictPass) || !strings.Contains(prompt, VerdictFail) {
		t.Errorf("prompt should contain %s/%s verdict labels", VerdictPass, VerdictFail)
	}
}

func TestWriteIssueMarkdown(t *testing.T) {
	tmp := t.TempDir()
	issue := &jira.Issue{
		Key:              "MAINT-99",
		Summary:          "Fix the broken thing",
		Status:           "In Progress",
		Priority:         "High",
		Reporter:         "Alice",
		Assignee:         "Bob",
		Components:       []string{"Backend", "Frontend"},
		AffectedVersions: []string{"2.0", "2.1"},
		FixVersions:      []string{"2.2"},
		Labels:           []string{"bug", "regression"},
		ExtraFields:      []jira.FieldValue{{Field: "Customers", Value: "Acme Corp"}},
		Description:      "This is the description.",
		Comments: []jira.Comment{
			{Author: "Charlie", Created: "2024-01-01T10:00:00Z", Body: "Please fix ASAP."},
		},
	}

	path := filepath.Join(tmp, "issue-MAINT-99.md")
	if err := writeIssueMarkdown(path, issue); err != nil {
		t.Fatalf("writeIssueMarkdown() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"# Jira Issue: MAINT-99",
		"Fix the broken thing",
		"In Progress",
		"High",
		"Alice",
		"Bob",
		"Backend, Frontend",
		"Acme Corp",
		"2.0, 2.1",
		"2.2",
		"bug, regression",
		"This is the description.",
		"### Charlie (2024-01-01T10:00:00Z)",
		"Please fix ASAP.",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected output to contain %q\nfull output:\n%s", check, content)
		}
	}
}

func TestWriteIssueMarkdown_NoComments(t *testing.T) {
	tmp := t.TempDir()
	issue := &jira.Issue{
		Key:     "MAINT-1",
		Summary: "No comments here",
	}

	path := filepath.Join(tmp, "issue-MAINT-1.md")
	if err := writeIssueMarkdown(path, issue); err != nil {
		t.Fatalf("writeIssueMarkdown() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "## Comments") {
		t.Error("expected no Comments section when there are no comments")
	}
}

func TestWriteReport(t *testing.T) {
	tmp := t.TempDir()
	r := Result{
		IssueKey:  "MAINT-55",
		Summary:   "Some maintenance issue",
		TriagedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Report:    "✅ Everything looks good.",
	}

	path := filepath.Join(tmp, "report-MAINT-55.md")
	if err := writeReport(path, r); err != nil {
		t.Fatalf("writeReport() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# Triage Report: MAINT-55") {
		t.Error("expected report header with issue key")
	}
	if !strings.Contains(content, "Some maintenance issue") {
		t.Error("expected summary in report")
	}
	if !strings.Contains(content, "2024-06-01T12:00:00Z") {
		t.Error("expected triaged-at timestamp in report")
	}
	if !strings.Contains(content, "✅ Everything looks good.") {
		t.Error("expected report body content")
	}
}

func TestPrintResults_MachineReadableJSON(t *testing.T) {
	var buf bytes.Buffer
	r := Result{
		IssueKey:  "MAINT-1",
		Summary:   "My summary",
		TriagedAt: time.Now(),
		Report:    "All good.",
		Evaluation: &Evaluation{
			Decision:   DecisionAccept,
			Confidence: 0.9,
		},
	}
	printResults(&buf, []Result{r})

	var batch TriageOutcomeBatch
	if err := json.Unmarshal(buf.Bytes(), &batch); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if batch.SchemaVersion == "" {
		t.Error("SchemaVersion should not be empty")
	}
	if len(batch.Issues) != 1 {
		t.Fatalf("Issues len = %d, want 1", len(batch.Issues))
	}
	decoded := batch.Issues[0]
	if decoded.IssueKey != "MAINT-1" {
		t.Errorf("IssueKey = %q, want %q", decoded.IssueKey, "MAINT-1")
	}
	if decoded.Summary != "My summary" {
		t.Errorf("Summary = %q, want %q", decoded.Summary, "My summary")
	}
	if decoded.Decision != DecisionAccept {
		t.Errorf("Decision = %q, want %q", decoded.Decision, DecisionAccept)
	}
	if decoded.Confidence != 0.9 {
		t.Errorf("Confidence = %g, want 0.9", decoded.Confidence)
	}
}

func TestPrintResults_RequestInfoDecision(t *testing.T) {
	var buf bytes.Buffer
	r := Result{
		IssueKey: "MAINT-2",
		Summary:  "Needs more info",
		Evaluation: &Evaluation{
			Decision:   DecisionRequestInfo,
			Confidence: 0.7,
			Questions:  []string{"What version are you on?", "Can you reproduce this?"},
		},
	}
	printResults(&buf, []Result{r})

	var batch TriageOutcomeBatch
	if err := json.Unmarshal(buf.Bytes(), &batch); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(batch.Issues) != 1 {
		t.Fatalf("Issues len = %d, want 1", len(batch.Issues))
	}
	decoded := batch.Issues[0]
	if decoded.Decision != DecisionRequestInfo {
		t.Errorf("Decision = %q, want %q", decoded.Decision, DecisionRequestInfo)
	}
	if len(decoded.Questions) != 2 {
		t.Errorf("Questions len = %d, want 2", len(decoded.Questions))
	}
}

func TestPrintResults_RejectDecision(t *testing.T) {
	var buf bytes.Buffer
	r := Result{
		IssueKey: "MAINT-3",
		Summary:  "Feature request disguised as bug",
		Evaluation: &Evaluation{
			Decision:        DecisionReject,
			Confidence:      0.95,
			RejectionReason: "This is a feature request, not a maintenance issue.",
		},
	}
	printResults(&buf, []Result{r})

	var batch TriageOutcomeBatch
	if err := json.Unmarshal(buf.Bytes(), &batch); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(batch.Issues) != 1 {
		t.Fatalf("Issues len = %d, want 1", len(batch.Issues))
	}
	decoded := batch.Issues[0]
	if decoded.Decision != DecisionReject {
		t.Errorf("Decision = %q, want %q", decoded.Decision, DecisionReject)
	}
	if decoded.RejectionReason == "" {
		t.Error("expected non-empty rejection reason")
	}
}

func TestPrintResults_WithError(t *testing.T) {
	var buf bytes.Buffer
	r := Result{
		IssueKey: "MAINT-4",
		Error:    "something went wrong",
	}
	printResults(&buf, []Result{r})

	var batch TriageOutcomeBatch
	if err := json.Unmarshal(buf.Bytes(), &batch); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(batch.Issues) != 1 {
		t.Fatalf("Issues len = %d, want 1", len(batch.Issues))
	}
	if batch.Issues[0].Error != "something went wrong" {
		t.Errorf("Error = %q, want %q", batch.Issues[0].Error, "something went wrong")
	}
}

func TestPrintResults_NoEvaluation_ErrorInOutput(t *testing.T) {
	var buf bytes.Buffer
	r := Result{
		IssueKey: "MAINT-5",
		Summary:  "Raw output issue",
		Report:   "Some raw agent output that couldn't be parsed.",
	}
	printResults(&buf, []Result{r})

	var batch TriageOutcomeBatch
	if err := json.Unmarshal(buf.Bytes(), &batch); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(batch.Issues) != 1 {
		t.Fatalf("Issues len = %d, want 1", len(batch.Issues))
	}
	if batch.Issues[0].Error == "" {
		t.Error("expected error message when evaluation is nil")
	}
}

func TestPrintResults_MultipleIssues(t *testing.T) {
	var buf bytes.Buffer
	results := []Result{
		{IssueKey: "MAINT-10", Summary: "First issue", Evaluation: &Evaluation{Decision: DecisionAccept, Confidence: 0.9, Verdict: VerdictPass}},
		{IssueKey: "MAINT-11", Summary: "Second issue", Evaluation: &Evaluation{Decision: DecisionRequestInfo, Confidence: 0.7, Verdict: VerdictFail, Questions: []string{"What version?"}}},
		{IssueKey: "MAINT-12", Error: "fetch failed"},
	}
	printResults(&buf, results)

	var batch TriageOutcomeBatch
	if err := json.Unmarshal(buf.Bytes(), &batch); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if batch.SchemaVersion == "" {
		t.Error("SchemaVersion should not be empty")
	}
	if len(batch.Issues) != 3 {
		t.Fatalf("Issues len = %d, want 3", len(batch.Issues))
	}
	if batch.Issues[0].IssueKey != "MAINT-10" {
		t.Errorf("Issues[0].IssueKey = %q, want %q", batch.Issues[0].IssueKey, "MAINT-10")
	}
	if batch.Issues[1].IssueKey != "MAINT-11" {
		t.Errorf("Issues[1].IssueKey = %q, want %q", batch.Issues[1].IssueKey, "MAINT-11")
	}
	if batch.Issues[2].Error != "fetch failed" {
		t.Errorf("Issues[2].Error = %q, want %q", batch.Issues[2].Error, "fetch failed")
	}
}

func TestRunConcurrent_OrderAndResults(t *testing.T) {
	keys := []string{"A", "B", "C", "D", "E"}
	results := runConcurrent(keys, 3, func(key string) Result {
		return Result{IssueKey: key}
	})

	if len(results) != len(keys) {
		t.Fatalf("got %d results, want %d", len(results), len(keys))
	}
	for i, r := range results {
		if r.IssueKey != keys[i] {
			t.Errorf("results[%d].IssueKey = %q, want %q", i, r.IssueKey, keys[i])
		}
	}
}

func TestRunConcurrent_BoundsConcurrency(t *testing.T) {
	const cap = 3
	keys := make([]string, 9)
	for i := range keys {
		keys[i] = fmt.Sprintf("KEY-%d", i)
	}

	var active atomic.Int64
	var mu sync.Mutex
	var maxSeen int64

	runConcurrent(keys, cap, func(key string) Result {
		cur := active.Add(1)
		mu.Lock()
		if cur > maxSeen {
			maxSeen = cur
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return Result{IssueKey: key}
	})

	if maxSeen > cap {
		t.Errorf("peak concurrency = %d, want <= %d", maxSeen, cap)
	}
	if maxSeen < 2 {
		t.Errorf("peak concurrency = %d; worker pool not utilized", maxSeen)
	}
}

func TestRunConcurrent_SingleWorker(t *testing.T) {
	keys := []string{"X", "Y", "Z"}
	var order []string
	var mu sync.Mutex

	runConcurrent(keys, 1, func(key string) Result {
		mu.Lock()
		order = append(order, key)
		mu.Unlock()
		return Result{IssueKey: key}
	})

	if len(order) != len(keys) {
		t.Fatalf("got %d calls, want %d", len(order), len(keys))
	}
}
