package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codcod/maints-triage/internal/agent"
	"github.com/codcod/maints-triage/internal/jira"
)

// aiResponse is the JSON structure the AI is asked to return.
type aiResponse struct {
	Tags       []string `json:"tags"`
	Confidence float64  `json:"confidence"`
	Reasoning  string   `json:"reasoning"`
}

// resolveWithAI uses cursor-agent to determine the appropriate repos.yaml tags
// for the given JIRA issue when no static rule matched.
func (r *Resolver) resolveWithAI(ctx context.Context, issue *jira.Issue) (Resolution, error) {
	prompt := r.buildAIPrompt(issue)
	output, err := agent.Run(ctx, prompt, agent.Options{
		APIKey: r.opts.AIAPIKey,
		Model:  r.opts.AIModel,
	})
	if err != nil {
		return Resolution{}, fmt.Errorf("cursor-agent: %w", err)
	}
	resp, err := parseAIResponse(output)
	if err != nil {
		return Resolution{}, fmt.Errorf("parse AI response: %w", err)
	}
	return Resolution{
		Tags:        resp.Tags,
		Source:      SourceAIFallback,
		Confidence:  resp.Confidence,
		AIReasoning: resp.Reasoning,
	}, nil
}

// buildAIPrompt constructs the cursor-agent prompt for repository resolution.
func (r *Resolver) buildAIPrompt(issue *jira.Issue) string {
	var sb strings.Builder

	sb.WriteString("You are a repository resolver for a software maintenance team.\n")
	sb.WriteString("Given a JIRA maintenance ticket, identify the most likely affected\n")
	sb.WriteString("repository (or repositories) by selecting matching tags from the\n")
	sb.WriteString("provided repos.yaml catalog.\n\n")

	sb.WriteString("## JIRA Ticket\n\n")
	fmt.Fprintf(&sb, "**Key:** %s\n", issue.Key)
	fmt.Fprintf(&sb, "**Summary:** %s\n", issue.Summary)
	fmt.Fprintf(&sb, "**Priority:** %s\n", issue.Priority)
	if len(issue.Components) > 0 {
		fmt.Fprintf(&sb, "**Components:** %s\n", strings.Join(issue.Components, ", "))
	}
	if len(issue.Labels) > 0 {
		fmt.Fprintf(&sb, "**Labels:** %s\n", strings.Join(issue.Labels, ", "))
	}
	if len(issue.AffectedVersions) > 0 {
		fmt.Fprintf(&sb, "**Affected versions:** %s\n", strings.Join(issue.AffectedVersions, ", "))
	}
	for _, fv := range issue.ExtraFields {
		if fv.Value != "" {
			fmt.Fprintf(&sb, "**%s:** %s\n", fv.Field, fv.Value)
		}
	}
	if issue.Description != "" {
		sb.WriteString("\n**Description:**\n")
		desc := issue.Description
		if len(desc) > 2000 {
			desc = desc[:2000] + "\n[truncated]"
		}
		sb.WriteString(desc)
		sb.WriteString("\n")
	}

	if r.opts.ReposYAML != "" {
		sb.WriteString("\n## Repository Catalog (repos.yaml)\n\n")
		sb.WriteString("```yaml\n")
		sb.WriteString(r.opts.ReposYAML)
		sb.WriteString("\n```\n")
	}

	sb.WriteString(`
## Instructions

Analyze the ticket and return a JSON object with:
- "tags": array of repos.yaml tags that best identify the affected repository or repositories.
  Only use tags that appear in the catalog above.
- "confidence": float from 0.0 to 1.0 representing how confident you are in the selection.
- "reasoning": one or two sentences explaining which signals led to your selection.

Return ONLY the JSON object, no other text. Example:
{"tags": ["android", "flow"], "confidence": 0.85, "reasoning": "The ticket mentions Android and affects the Flow Mobile sub-component."}
`)
	return sb.String()
}

// parseAIResponse extracts an aiResponse from the agent's raw output.
// It accepts a bare JSON object or one embedded in a ```json … ``` fence.
func parseAIResponse(raw string) (aiResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return aiResponse{}, fmt.Errorf("empty agent response")
	}

	var resp aiResponse
	if err := json.Unmarshal([]byte(raw), &resp); err == nil && len(resp.Tags) > 0 {
		return resp, nil
	}

	if block := extractJSONFence(raw); block != "" {
		if err := json.Unmarshal([]byte(block), &resp); err == nil && len(resp.Tags) > 0 {
			return resp, nil
		}
	}

	return aiResponse{}, fmt.Errorf("could not parse tags from agent output: %q", truncate(raw, 200))
}

// extractJSONFence pulls the content of the first ```json…``` code fence from s.
func extractJSONFence(s string) string {
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
