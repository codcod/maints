package triage

import (
	"strings"
	"testing"
)

// --- parseEvaluation ---

func TestParseEvaluation_ValidJSON(t *testing.T) {
	raw := `{
		"items": [
			{"id":1,"title":"Priority","status":"complete","evidence":"Priority: Major","reasoning":"Default priority, no justification required."},
			{"id":2,"title":"Component","status":"missing","evidence":"","reasoning":"No component listed."}
		],
		"summary": "Component is missing.",
		"verdict": "FAIL_CHECKLIST_COMPLIANCE",
		"review_required": false,
		"decision": "request_info",
		"confidence": 0.8,
		"questions": ["Which component is affected?"]
	}`

	e, err := parseEvaluation(raw)
	if err != nil {
		t.Fatalf("parseEvaluation() error = %v", err)
	}
	if e == nil {
		t.Fatal("parseEvaluation() returned nil for valid JSON")
	}
	if len(e.Items) != 2 {
		t.Errorf("Items len = %d, want 2", len(e.Items))
	}
	if e.Verdict != VerdictFail {
		t.Errorf("Verdict = %q, want %s", e.Verdict, VerdictFail)
	}
	if e.Items[0].Status != StatusComplete {
		t.Errorf("Items[0].Status = %q, want complete", e.Items[0].Status)
	}
	if e.Items[1].Status != StatusMissing {
		t.Errorf("Items[1].Status = %q, want missing", e.Items[1].Status)
	}
	if e.Decision != DecisionRequestInfo {
		t.Errorf("Decision = %q, want %q", e.Decision, DecisionRequestInfo)
	}
	if e.Confidence != 0.8 {
		t.Errorf("Confidence = %g, want 0.8", e.Confidence)
	}
}

func TestParseEvaluation_JSONInCodeFence(t *testing.T) {
	raw := "Here is the evaluation:\n\n```json\n" +
		`{"items":[{"id":1,"title":"Priority","status":"complete","evidence":"Major","reasoning":"ok"}],"summary":"All good.","verdict":"PASS_CHECKLIST_COMPLIANCE","review_required":false,"decision":"accept","confidence":0.95}` +
		"\n```\n\nDone."

	e, err := parseEvaluation(raw)
	if err != nil {
		t.Fatalf("parseEvaluation() error = %v", err)
	}
	if e == nil {
		t.Fatal("parseEvaluation() returned nil for fenced JSON")
	}
	if e.Verdict != VerdictPass {
		t.Errorf("Verdict = %q, want %s", e.Verdict, VerdictPass)
	}
	if e.Decision != DecisionAccept {
		t.Errorf("Decision = %q, want %q", e.Decision, DecisionAccept)
	}
}

func TestParseEvaluation_PlainText_ReturnsNil(t *testing.T) {
	e, err := parseEvaluation("This is a plain text report with no JSON.")
	if err != nil {
		t.Fatalf("parseEvaluation() unexpected error: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil for plain text input, got %+v", e)
	}
}

func TestParseEvaluation_EmptyString_ReturnsNil(t *testing.T) {
	e, err := parseEvaluation("   ")
	if err != nil {
		t.Fatalf("parseEvaluation() unexpected error: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil for empty input, got %+v", e)
	}
}

func TestParseEvaluation_JSONWithNoItems_ReturnsNil(t *testing.T) {
	// Valid JSON but does not look like an Evaluation (no items, no verdict).
	e, err := parseEvaluation(`{"foo":"bar"}`)
	if err != nil {
		t.Fatalf("parseEvaluation() unexpected error: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil for JSON without items/verdict, got %+v", e)
	}
}

func TestParseEvaluation_JSONWithEmptyVerdict_ReturnsNil(t *testing.T) {
	raw := `{"items":[{"id":1,"title":"T","status":"complete","evidence":"e","reasoning":"r"}],"summary":"","verdict":"","review_required":false,"decision":"accept","confidence":0.9}`
	e, err := parseEvaluation(raw)
	if err != nil {
		t.Fatalf("parseEvaluation() unexpected error: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil when verdict is empty, got %+v", e)
	}
}

func TestParseEvaluation_JSONWithEmptyDecision_ReturnsNil(t *testing.T) {
	raw := `{"items":[{"id":1,"title":"T","status":"complete","evidence":"e","reasoning":"r"}],"summary":"ok","verdict":"PASS_CHECKLIST_COMPLIANCE","review_required":false,"decision":"","confidence":0.9}`
	e, err := parseEvaluation(raw)
	if err != nil {
		t.Fatalf("parseEvaluation() unexpected error: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil when decision is empty, got %+v", e)
	}
}

func TestParseEvaluation_JSONWithOutOfRangeConfidence_ReturnsNil(t *testing.T) {
	raw := `{"items":[{"id":1,"title":"T","status":"complete","evidence":"e","reasoning":"r"}],"summary":"ok","verdict":"PASS_CHECKLIST_COMPLIANCE","review_required":false,"decision":"accept","confidence":1.5}`
	e, err := parseEvaluation(raw)
	if err != nil {
		t.Fatalf("parseEvaluation() unexpected error: %v", err)
	}
	if e != nil {
		t.Errorf("expected nil when confidence is out of range, got %+v", e)
	}
}

func TestParseEvaluation_AllStatusValues(t *testing.T) {
	raw := `{
		"items": [
			{"id":1,"title":"A","status":"complete","evidence":"q","reasoning":"r"},
			{"id":2,"title":"B","status":"partial","evidence":"q","reasoning":"r"},
			{"id":3,"title":"C","status":"missing","evidence":"","reasoning":"r"},
			{"id":4,"title":"D","status":"na","evidence":"","reasoning":"r"}
		],
		"summary": "two issues",
		"verdict": "FAIL_CHECKLIST_COMPLIANCE",
		"review_required": false,
		"decision": "request_info",
		"confidence": 0.7,
		"questions": ["What is the expected behaviour?"]
	}`

	e, err := parseEvaluation(raw)
	if err != nil || e == nil {
		t.Fatalf("parseEvaluation() error = %v, e = %v", err, e)
	}
	want := []ItemStatus{StatusComplete, StatusPartial, StatusMissing, StatusNA}
	for i, w := range want {
		if e.Items[i].Status != w {
			t.Errorf("Items[%d].Status = %q, want %q", i, e.Items[i].Status, w)
		}
	}
}

// --- validateEvaluation ---

func TestValidateEvaluation_Clean(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusComplete, Evidence: "Priority: Major", Reasoning: "ok"},
			{ID: 2, Title: "Component", Status: StatusNA, Evidence: "", Reasoning: "not applicable"},
		},
		Summary:    "All good.",
		Verdict:    VerdictPass,
		Decision:   DecisionAccept,
		Confidence: 0.95,
	}
	warnings := validateEvaluation(e)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for clean evaluation, got: %v", warnings)
	}
}

func TestValidateEvaluation_PassWithMissingItem(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusMissing, Evidence: "", Reasoning: "not present"},
		},
		Verdict:    VerdictPass,
		Decision:   DecisionAccept,
		Confidence: 0.5,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, VerdictPass) && strings.Contains(w, "missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about %s with missing items, got: %v", VerdictPass, warnings)
	}
}

func TestValidateEvaluation_FailWithAllComplete(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusComplete, Evidence: "e", Reasoning: "r"},
		},
		Verdict:         VerdictFail,
		Decision:        DecisionReject,
		Confidence:      0.6,
		RejectionReason: "Does not meet criteria.",
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, VerdictFail) && strings.Contains(w, "complete") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about %s with all complete items, got: %v", VerdictFail, warnings)
	}
}

func TestValidateEvaluation_MissingEvidenceForNonNA(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusComplete, Evidence: "", Reasoning: "ok"},
		},
		Verdict:    VerdictPass,
		Decision:   DecisionAccept,
		Confidence: 0.9,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "evidence") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about missing evidence, got: %v", warnings)
	}
}

func TestValidateEvaluation_UnrecognisedStatus(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: "unknown", Evidence: "e", Reasoning: "r"},
		},
		Verdict:         VerdictFail,
		Decision:        DecisionReject,
		Confidence:      0.5,
		RejectionReason: "Not valid.",
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "unrecognised status") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about unrecognised status, got: %v", warnings)
	}
}

func TestValidateEvaluation_UnrecognisedVerdict(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusComplete, Evidence: "e", Reasoning: "r"}},
		Verdict:    "MAYBE",
		Decision:   DecisionAccept,
		Confidence: 0.5,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "unrecognised verdict") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about unrecognised verdict, got: %v", warnings)
	}
}

func TestValidateEvaluation_UnrecognisedDecision(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusComplete, Evidence: "e", Reasoning: "r"}},
		Verdict:    VerdictPass,
		Decision:   "maybe",
		Confidence: 0.5,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "unrecognised decision") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about unrecognised decision, got: %v", warnings)
	}
}

func TestValidateEvaluation_RequestInfoWithoutQuestions(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusMissing, Evidence: "", Reasoning: "r"}},
		Verdict:    VerdictFail,
		Decision:   DecisionRequestInfo,
		Confidence: 0.7,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "no questions") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about missing questions for request_info, got: %v", warnings)
	}
}

func TestValidateEvaluation_RejectWithoutReason(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusMissing, Evidence: "", Reasoning: "r"}},
		Verdict:    VerdictFail,
		Decision:   DecisionReject,
		Confidence: 0.8,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "no rejection_reason") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about missing rejection_reason, got: %v", warnings)
	}
}

func TestValidateEvaluation_PassWithRejectDecision(t *testing.T) {
	e := &Evaluation{
		Items:           []ChecklistItem{{ID: 1, Title: "T", Status: StatusComplete, Evidence: "e", Reasoning: "r"}},
		Verdict:         VerdictPass,
		Decision:        DecisionReject,
		Confidence:      0.5,
		RejectionReason: "Not valid.",
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, VerdictPass) && strings.Contains(w, "reject") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about %s verdict with reject decision, got: %v", VerdictPass, warnings)
	}
}

func TestValidateEvaluation_PassWithRequestInfoDecision(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusComplete, Evidence: "e", Reasoning: "r"}},
		Verdict:    VerdictPass,
		Decision:   DecisionRequestInfo,
		Confidence: 0.5,
		Questions:  []string{"What?"},
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, VerdictPass) && strings.Contains(w, "request_info") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about %s verdict with request_info decision, got: %v", VerdictPass, warnings)
	}
}

func TestValidateEvaluation_FailWithAcceptDecision_NoWarning(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusMissing, Evidence: "", Reasoning: "r"}},
		Verdict:    VerdictFail,
		Decision:   DecisionAccept,
		Confidence: 0.7,
	}
	warnings := validateEvaluation(e)
	for _, w := range warnings {
		if strings.Contains(w, VerdictFail) && strings.Contains(w, "accept") {
			t.Errorf("should not warn about FAIL+accept (valid under new model), got: %v", warnings)
		}
	}
}

func TestValidateEvaluation_OutOfRangeConfidence(t *testing.T) {
	e := &Evaluation{
		Items:      []ChecklistItem{{ID: 1, Title: "T", Status: StatusComplete, Evidence: "e", Reasoning: "r"}},
		Verdict:    VerdictPass,
		Decision:   DecisionAccept,
		Confidence: 1.5,
	}
	warnings := validateEvaluation(e)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "out of range") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about out-of-range confidence, got: %v", warnings)
	}
}

// --- renderEvaluationMarkdown ---

func TestRenderEvaluationMarkdown_Structure(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusComplete, Evidence: "Priority is Major.", Reasoning: "Default priority."},
			{ID: 2, Title: "Component", Status: StatusMissing, Evidence: "", Reasoning: "Nothing listed."},
			{ID: 3, Title: "Environment", Status: StatusNA, Evidence: "", Reasoning: "N/A for this issue type."},
		},
		Summary:    "Component is missing.",
		Verdict:    VerdictFail,
		Decision:   DecisionRequestInfo,
		Confidence: 0.75,
		Questions:  []string{"Which component is affected?"},
	}

	out := renderEvaluationMarkdown(e)

	checks := []string{
		"## Checklist Evaluation",
		"### 1. Priority — Complete",
		"> Priority is Major.",
		"Default priority.",
		"### 2. Component — Missing",
		"Nothing listed.",
		"### 3. Environment — N/A",
		"## Summary of Gaps",
		"| 2 | Component | Missing |",
		"## Overall Verdict: **" + VerdictFail + "**",
		"Component is missing.",
		"## Triage Decision",
		"**Decision:** request_info",
		"**Confidence:** 0.75",
		"**Questions for reporter:**",
		"- Which component is affected?",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("expected output to contain %q\nfull output:\n%s", check, out)
		}
	}
}

func TestRenderEvaluationMarkdown_NoGaps_NoGapTable(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusComplete, Evidence: "Major", Reasoning: "ok"},
		},
		Summary:    "All good.",
		Verdict:    VerdictPass,
		Decision:   DecisionAccept,
		Confidence: 0.95,
	}

	out := renderEvaluationMarkdown(e)

	if strings.Contains(out, "Summary of Gaps") {
		t.Error("expected no gap table when all items are complete")
	}
	if !strings.Contains(out, "## Overall Verdict: **"+VerdictPass+"**") {
		t.Errorf("expected %s verdict in output", VerdictPass)
	}
	if !strings.Contains(out, "**Decision:** accept") {
		t.Error("expected decision in output")
	}
	if !strings.Contains(out, "**Confidence:** 0.95") {
		t.Error("expected confidence in output")
	}
}

func TestRenderEvaluationMarkdown_PartialInGapTable(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 7, Title: "Expected vs Actual", Status: StatusPartial, Evidence: "Actual described.", Reasoning: "Expected not stated."},
		},
		Summary:    "Expected behavior missing.",
		Verdict:    VerdictFail,
		Decision:   DecisionRequestInfo,
		Confidence: 0.8,
		Questions:  []string{"What is the expected behaviour?"},
	}

	out := renderEvaluationMarkdown(e)

	if !strings.Contains(out, "Partial") {
		t.Error("expected partial status label in output")
	}
	if !strings.Contains(out, "| 7 | Expected vs Actual | Partial |") {
		t.Errorf("expected partial item in gap table\nfull output:\n%s", out)
	}
}

func TestRenderEvaluationMarkdown_RejectDecision(t *testing.T) {
	e := &Evaluation{
		Items: []ChecklistItem{
			{ID: 1, Title: "Priority", Status: StatusMissing, Evidence: "", Reasoning: "Not provided."},
		},
		Summary:         "Issue is out of scope.",
		Verdict:         VerdictFail,
		Decision:        DecisionReject,
		Confidence:      0.9,
		RejectionReason: "This is a feature request, not a maintenance issue.",
	}

	out := renderEvaluationMarkdown(e)

	if !strings.Contains(out, "**Decision:** reject") {
		t.Error("expected decision in output")
	}
	if !strings.Contains(out, "**Rejection reason:** This is a feature request, not a maintenance issue.") {
		t.Errorf("expected rejection reason in output\nfull output:\n%s", out)
	}
	if strings.Contains(out, "Questions for reporter") {
		t.Error("expected no questions section for reject decision")
	}
}

// --- extractJSONBlock ---

func TestExtractJSONBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fence",
			input: "plain text",
			want:  "",
		},
		{
			name:  "fenced block",
			input: "before\n```json\n{\"key\":\"val\"}\n```\nafter",
			want:  `{"key":"val"}`,
		},
		{
			name:  "unclosed fence returns empty",
			input: "```json\n{\"key\":\"val\"}",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONBlock(tt.input)
			if got != tt.want {
				t.Errorf("extractJSONBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}
