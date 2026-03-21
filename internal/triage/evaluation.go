package triage

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ItemStatus is the evaluation result for a single checklist item.
type ItemStatus string

const (
	StatusComplete ItemStatus = "complete"
	StatusPartial  ItemStatus = "partial"
	StatusMissing  ItemStatus = "missing"
	StatusNA       ItemStatus = "na"
)

// Verdict values for an Evaluation.
const (
	VerdictPass = "PASS_CHECKLIST_COMPLIANCE"
	VerdictFail = "FAIL_CHECKLIST_COMPLIANCE"
)

// Decision is the triage workflow outcome for a Jira ticket.
type Decision string

const (
	DecisionAccept      Decision = "accept"
	DecisionRequestInfo Decision = "request_info"
	DecisionReject      Decision = "reject"
)

// ChecklistItem is the structured result for a single checklist item.
type ChecklistItem struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Status    ItemStatus `json:"status"`
	Evidence  string     `json:"evidence"`  // exact quote from the issue; empty for "na"
	Reasoning string     `json:"reasoning"` // one-sentence justification
}

// Evaluation is the structured output from the agent for a full triage run.
// It is populated when the agent returns well-formed JSON; callers fall back
// to the raw text report when it is nil.
type Evaluation struct {
	Items           []ChecklistItem `json:"items"`
	Summary         string          `json:"summary"`
	Verdict         string          `json:"verdict"`                    // VerdictPass or VerdictFail
	ReviewRequired  bool            `json:"review_required"`            // true when the AI flags uncertainty
	Decision        Decision        `json:"decision"`                   // "accept", "request_info", or "reject"
	Confidence      float64         `json:"confidence"`                 // 0.0–1.0
	Questions       []string        `json:"questions,omitempty"`        // required when decision is "request_info"
	RejectionReason string          `json:"rejection_reason,omitempty"` // required when decision is "reject"
}

// parseEvaluation attempts to extract a structured Evaluation from the raw
// agent output. It tries, in order:
//  1. The full output parsed directly as a JSON object.
//  2. A ```json…``` fenced block embedded anywhere in the output.
//
// Returns (nil, nil) when the output is not parseable as a valid Evaluation,
// allowing callers to degrade gracefully to the raw text.
func parseEvaluation(raw string) (*Evaluation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	if e := tryUnmarshalEvaluation(raw); e != nil {
		return e, nil
	}

	if block := extractJSONBlock(raw); block != "" {
		if e := tryUnmarshalEvaluation(block); e != nil {
			return e, nil
		}
	}

	return nil, nil
}

// tryUnmarshalEvaluation unmarshals s into an Evaluation and returns it only
// when the result has at least one item, a non-empty verdict, a valid decision,
// and a confidence value in [0.0, 1.0].
func tryUnmarshalEvaluation(s string) *Evaluation {
	var e Evaluation
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		return nil
	}
	if len(e.Items) == 0 || e.Verdict == "" {
		return nil
	}
	if e.Decision == "" || e.Confidence < 0.0 || e.Confidence > 1.0 {
		return nil
	}
	return &e
}

// extractJSONBlock pulls the content of the first ```json…``` code fence from s.
func extractJSONBlock(s string) string {
	const openFence = "```json"
	const closeFence = "```"

	start := strings.Index(s, openFence)
	if start == -1 {
		return ""
	}
	start += len(openFence)

	end := strings.Index(s[start:], closeFence)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(s[start : start+end])
}

// validateEvaluation checks that an Evaluation is internally consistent.
// It returns a slice of human-readable warning strings; an empty slice means
// no issues were found.
func validateEvaluation(e *Evaluation) []string {
	warnings, hasProblem := validateItems(e.Items)
	warnings = append(warnings, validateVerdict(e.Verdict, hasProblem)...)
	warnings = append(warnings, validateDecision(e)...)
	return warnings
}

func validateItems(items []ChecklistItem) (warnings []string, hasProblem bool) {
	for _, item := range items {
		switch item.Status {
		case StatusComplete, StatusPartial, StatusMissing, StatusNA:
		default:
			warnings = append(warnings,
				fmt.Sprintf("item %d (%q) has unrecognised status %q", item.ID, item.Title, item.Status))
		}
		if item.Status != StatusNA && item.Evidence == "" {
			warnings = append(warnings,
				fmt.Sprintf("item %d (%q) is %s but has no evidence quote", item.ID, item.Title, item.Status))
		}
		if item.Status == StatusPartial || item.Status == StatusMissing {
			hasProblem = true
		}
	}
	return warnings, hasProblem
}

func validateVerdict(verdict string, hasProblem bool) []string {
	var warnings []string
	switch verdict {
	case VerdictPass, VerdictFail:
	default:
		warnings = append(warnings,
			fmt.Sprintf("unrecognised verdict %q (expected %s or %s)", verdict, VerdictPass, VerdictFail))
	}
	if verdict == VerdictPass && hasProblem {
		warnings = append(warnings, "verdict is "+VerdictPass+" but one or more items are partial or missing")
	}
	if verdict == VerdictFail && !hasProblem {
		warnings = append(warnings, "verdict is "+VerdictFail+" but all items are complete or N/A")
	}
	return warnings
}

func validateDecision(e *Evaluation) []string {
	var warnings []string
	switch e.Decision {
	case DecisionAccept, DecisionRequestInfo, DecisionReject:
	default:
		warnings = append(warnings,
			fmt.Sprintf("unrecognised decision %q (expected accept, request_info, or reject)", e.Decision))
	}
	if e.Confidence < 0.0 || e.Confidence > 1.0 {
		warnings = append(warnings,
			fmt.Sprintf("confidence %g is out of range [0.0, 1.0]", e.Confidence))
	}
	if e.Decision == DecisionRequestInfo && len(e.Questions) == 0 {
		warnings = append(warnings, "decision is request_info but no questions provided")
	}
	if e.Decision == DecisionReject && e.RejectionReason == "" {
		warnings = append(warnings, "decision is reject but no rejection_reason provided")
	}
	if e.Verdict == VerdictPass && e.Decision == DecisionReject {
		warnings = append(warnings,
			"verdict is "+VerdictPass+" but decision is reject — if the issue is not a valid defect, the Issue Type Verification item should not be complete")
	}
	if e.Verdict == VerdictPass && e.Decision == DecisionRequestInfo {
		warnings = append(warnings,
			"verdict is "+VerdictPass+" but decision is request_info — all items passed so no info should be needed")
	}
	return warnings
}

// renderEvaluationMarkdown converts a structured Evaluation to the
// human-readable markdown that is written to the report body.
func renderEvaluationMarkdown(e *Evaluation) string {
	var sb strings.Builder

	sb.WriteString("## Checklist Evaluation\n\n")
	for _, item := range e.Items {
		fmt.Fprintf(&sb, "### %d. %s — %s\n\n", item.ID, item.Title, statusLabel(item.Status))
		if item.Evidence != "" {
			fmt.Fprintf(&sb, "> %s\n\n", item.Evidence)
		}
		if item.Reasoning != "" {
			sb.WriteString(item.Reasoning + "\n\n")
		}
	}

	// Gap table — only partial / missing items.
	var gaps []ChecklistItem
	for _, item := range e.Items {
		if item.Status == StatusPartial || item.Status == StatusMissing {
			gaps = append(gaps, item)
		}
	}
	if len(gaps) > 0 {
		sb.WriteString("---\n\n## Summary of Gaps\n\n")
		sb.WriteString("| # | Item | Status |\n")
		sb.WriteString("|---|------|--------|\n")
		for _, item := range gaps {
			fmt.Fprintf(&sb, "| %d | %s | %s |\n", item.ID, item.Title, statusLabel(item.Status))
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "---\n\n## Overall Verdict: **%s**\n\n", e.Verdict)
	if e.Summary != "" {
		sb.WriteString(e.Summary + "\n")
	}

	sb.WriteString(renderDecisionSection(e))

	return sb.String()
}

func renderDecisionSection(e *Evaluation) string {
	var sb strings.Builder
	sb.WriteString("\n---\n\n## Triage Decision\n\n")
	fmt.Fprintf(&sb, "**Decision:** %s\n\n", e.Decision)
	fmt.Fprintf(&sb, "**Confidence:** %.2f\n\n", e.Confidence)
	if e.ReviewRequired {
		sb.WriteString("**Review required:** Yes\n")
	} else {
		sb.WriteString("**Review required:** No\n")
	}
	if len(e.Questions) > 0 {
		sb.WriteString("\n**Questions for reporter:**\n\n")
		for _, q := range e.Questions {
			fmt.Fprintf(&sb, "- %s\n", q)
		}
	}
	if e.RejectionReason != "" {
		fmt.Fprintf(&sb, "\n**Rejection reason:** %s\n", e.RejectionReason)
	}
	return sb.String()
}

// statusLabel returns the display string for an ItemStatus.
func statusLabel(s ItemStatus) string {
	switch s {
	case StatusComplete:
		return "Complete"
	case StatusPartial:
		return "Partial"
	case StatusMissing:
		return "Missing"
	case StatusNA:
		return "N/A"
	default:
		return string(s)
	}
}
