// Package resolver maps JIRA issue metadata to repos.yaml tags.
//
// It uses a two-layer strategy:
//  1. Static rule matching against repo-mapping.json (fast, deterministic).
//  2. AI-assisted fallback via cursor-agent when no rule matches (requires CURSOR_API_KEY).
//
// Resolved tags are passed directly to `repos -t <tag>` so that the repos tool
// can filter to the correct repository set.
package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/codcod/maints-triage/internal/config"
	"github.com/codcod/maints-triage/internal/jira"
)

const defaultMappingFile = "repo-mapping.json"

// --- Config types -----------------------------------------------------------

// MappingMatch holds the criteria for a rule.
// All non-empty fields must be satisfied simultaneously (AND logic).
// An entirely empty MappingMatch matches every issue.
type MappingMatch struct {
	// Component matches when any element of issue.Components contains this string
	// (case-insensitive substring).
	Component string `json:"component,omitempty"`

	// MaintComponent matches the ExtraField named "Maint Component"
	// (case-insensitive substring).
	MaintComponent string `json:"maint_component,omitempty"`

	// SubComponent matches the ExtraField named "Maint Sub-component"
	// (case-insensitive substring).
	SubComponent string `json:"sub_component,omitempty"`

	// Labels requires ALL listed labels to be present on the issue
	// (case-insensitive exact match).
	Labels []string `json:"labels,omitempty"`

	// SummaryContains is a case-insensitive substring match against issue.Summary.
	SummaryContains string `json:"summary_contains,omitempty"`

	// AffectedVersion is a case-insensitive substring match against any
	// element of issue.AffectedVersions.
	AffectedVersion string `json:"affected_version,omitempty"`
}

// MappingRule maps a set of match criteria to a list of repos.yaml tags.
type MappingRule struct {
	// Description is an optional human-readable label used in logs and reports.
	Description string `json:"description,omitempty"`

	// Match holds the criteria that must all be satisfied for this rule to fire.
	Match MappingMatch `json:"match"`

	// Tags is the list of repos.yaml tags that identify the affected repositories.
	Tags []string `json:"tags"`
}

// MappingConfig is the top-level structure of repo-mapping.json.
type MappingConfig struct {
	// Rules is evaluated in order; the first match wins.
	Rules []MappingRule `json:"rules"`

	// DefaultTags are returned when no rule matches and AI fallback is disabled
	// or yields no result.
	DefaultTags []string `json:"default_tags,omitempty"`
}

// --- Resolution types -------------------------------------------------------

// Source identifies how a Resolution was determined.
type Source string

const (
	// SourceStaticRule means a rule in repo-mapping.json matched.
	SourceStaticRule Source = "static_rule"
	// SourceAIFallback means cursor-agent determined the tags.
	SourceAIFallback Source = "ai_fallback"
	// SourceDefault means neither a rule nor the AI produced a result;
	// the config's DefaultTags were used.
	SourceDefault Source = "default"
)

// Resolution is the output of resolving a JIRA issue to repos.yaml tags.
type Resolution struct {
	// Tags is the list of repos.yaml tags that identify the affected repositories.
	Tags []string `json:"tags"`

	// Source indicates how the resolution was determined.
	Source Source `json:"source"`

	// RuleDescription describes the matched rule (populated for SourceStaticRule).
	RuleDescription string `json:"rule_description,omitempty"`

	// Confidence is 0.0–1.0: always 1.0 for static rules, AI-reported for
	// ai_fallback, and 0.3 for default.
	Confidence float64 `json:"confidence"`

	// AIReasoning captures the AI's explanation when Source is SourceAIFallback.
	AIReasoning string `json:"ai_reasoning,omitempty"`
}

// Empty reports whether the resolution contains no tags.
func (r Resolution) Empty() bool {
	return len(r.Tags) == 0
}

// --- Options & Resolver -----------------------------------------------------

// Options configures a Resolver.
type Options struct {
	// MappingPath overrides the default repo-mapping.json lookup path.
	// When empty the file is searched in TRIAGE_HOME, then the current directory.
	MappingPath string

	// AIFallback enables cursor-agent fallback when no static rule matches.
	AIFallback bool

	// AIAPIKey is the cursor-agent API key. Required when AIFallback is true.
	AIAPIKey string

	// AIModel is the cursor-agent model to use (e.g. "claude-sonnet-4"). Optional.
	AIModel string

	// ReposYAML is the raw content of repos.yaml, passed to the AI as context so
	// it can choose tags that actually exist. When empty the AI still operates but
	// without the full catalog.
	ReposYAML string

	// MinConfidence is the minimum AI confidence score [0.0, 1.0] required to
	// accept an AI-generated resolution. Resolutions below this fall through to
	// DefaultTags. Defaults to 0.0 (accept any).
	MinConfidence float64
}

// Resolver resolves JIRA issues to repos.yaml tags.
type Resolver struct {
	cfg  MappingConfig
	opts Options
}

// New creates a Resolver by loading the mapping config.
// If the config file does not exist the resolver operates with an empty rule
// set (AI fallback or DefaultTags still apply).
func New(opts Options) (*Resolver, error) {
	cfg, err := loadMappingConfig(opts.MappingPath)
	if err != nil {
		return nil, err
	}
	return &Resolver{cfg: cfg, opts: opts}, nil
}

// Config returns the loaded MappingConfig. Useful for inspection and testing.
func (r *Resolver) Config() MappingConfig {
	return r.cfg
}

// Resolve returns the repos.yaml tags for the given JIRA issue.
//
// Resolution strategy (first success wins):
//  1. Static rules from repo-mapping.json (first matching rule).
//  2. AI-assisted fallback via cursor-agent (when AIFallback is true and
//     the result meets MinConfidence).
//  3. DefaultTags from the config.
//
// Returns an empty Resolution (Tags == nil) when none of the above succeed.
func (r *Resolver) Resolve(ctx context.Context, issue *jira.Issue) (Resolution, error) {
	// 1. Static rule matching.
	if rule, ok := r.firstMatchingRule(issue); ok {
		desc := rule.Description
		if desc == "" {
			desc = fmt.Sprintf("rule match for %q", issue.Key)
		}
		return Resolution{
			Tags:            rule.Tags,
			Source:          SourceStaticRule,
			RuleDescription: desc,
			Confidence:      1.0,
		}, nil
	}

	// 2. AI-assisted fallback.
	if r.opts.AIFallback && r.opts.AIAPIKey != "" {
		res, err := r.resolveWithAI(ctx, issue)
		if err == nil && !res.Empty() && res.Confidence >= r.opts.MinConfidence {
			return res, nil
		}
		// AI failure is non-fatal; continue to defaults.
	}

	// 3. Default tags.
	if len(r.cfg.DefaultTags) > 0 {
		return Resolution{
			Tags:       r.cfg.DefaultTags,
			Source:     SourceDefault,
			Confidence: 0.3,
		}, nil
	}

	// 4. No resolution possible.
	return Resolution{}, nil
}

// firstMatchingRule returns the first rule whose criteria all match the issue.
func (r *Resolver) firstMatchingRule(issue *jira.Issue) (MappingRule, bool) {
	for _, rule := range r.cfg.Rules {
		if ruleMatches(rule.Match, issue) {
			return rule, true
		}
	}
	return MappingRule{}, false
}

// --- Rule matching ----------------------------------------------------------

// ruleMatches reports whether all non-empty fields in m are satisfied by issue.
// An entirely empty MappingMatch matches every issue (useful as a catch-all rule).
func ruleMatches(m MappingMatch, issue *jira.Issue) bool {
	if m.Component != "" && !anyContainsFold(issue.Components, m.Component) {
		return false
	}
	if m.MaintComponent != "" {
		if !containsFold(extraFieldValue(issue, "Maint Component"), m.MaintComponent) {
			return false
		}
	}
	if m.SubComponent != "" {
		if !containsFold(extraFieldValue(issue, "Maint Sub-component"), m.SubComponent) {
			return false
		}
	}
	for _, label := range m.Labels {
		if !anyEqualFold(issue.Labels, label) {
			return false
		}
	}
	if m.SummaryContains != "" && !containsFold(issue.Summary, m.SummaryContains) {
		return false
	}
	if m.AffectedVersion != "" && !anyContainsFold(issue.AffectedVersions, m.AffectedVersion) {
		return false
	}
	return true
}

// extraFieldValue returns the Value of the first ExtraField whose Field name
// matches fieldName (case-insensitive), or an empty string when not found.
func extraFieldValue(issue *jira.Issue, fieldName string) string {
	for _, fv := range issue.ExtraFields {
		if strings.EqualFold(fv.Field, fieldName) {
			return fv.Value
		}
	}
	return ""
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func anyContainsFold(ss []string, substr string) bool {
	for _, s := range ss {
		if containsFold(s, substr) {
			return true
		}
	}
	return false
}

func anyEqualFold(ss []string, target string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

// --- Config loading ---------------------------------------------------------

// loadMappingConfig reads repo-mapping.json from, in priority order:
//  1. explicit path (non-empty).
//  2. $TRIAGE_HOME/repo-mapping.json.
//  3. ./repo-mapping.json (current directory).
//
// Returns an empty MappingConfig without error when the file does not exist.
func loadMappingConfig(explicit string) (MappingConfig, error) {
	path, err := resolveMappingPath(explicit)
	if err != nil {
		return MappingConfig{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return MappingConfig{}, nil
	}
	if err != nil {
		return MappingConfig{}, fmt.Errorf("read %q: %w", path, err)
	}
	var cfg MappingConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return MappingConfig{}, fmt.Errorf("parse %q: %w", path, err)
	}
	return cfg, nil
}

func resolveMappingPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	th, err := config.TriageHome()
	if err != nil {
		return "", err
	}
	thPath := filepath.Join(th, defaultMappingFile)
	if _, err := os.Stat(thPath); err == nil {
		return thPath, nil
	}
	return defaultMappingFile, nil
}
