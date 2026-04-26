package resolver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codcod/maints-triage/internal/jira"
)

// --- helpers ----------------------------------------------------------------

func makeIssue(opts ...func(*jira.Issue)) *jira.Issue {
	issue := &jira.Issue{
		Key:     "MAINT-1",
		Summary: "Test issue",
	}
	for _, o := range opts {
		o(issue)
	}
	return issue
}

func withComponent(c string) func(*jira.Issue) {
	return func(i *jira.Issue) { i.Components = append(i.Components, c) }
}

func withLabel(l string) func(*jira.Issue) {
	return func(i *jira.Issue) { i.Labels = append(i.Labels, l) }
}

func withAffectedVersion(v string) func(*jira.Issue) {
	return func(i *jira.Issue) { i.AffectedVersions = append(i.AffectedVersions, v) }
}

func withSummary(s string) func(*jira.Issue) {
	return func(i *jira.Issue) { i.Summary = s }
}

func withExtraField(field, value string) func(*jira.Issue) {
	return func(i *jira.Issue) {
		i.ExtraFields = append(i.ExtraFields, jira.FieldValue{Field: field, Value: value})
	}
}

func writeMappingConfig(t *testing.T, dir string, cfg MappingConfig) string {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(dir, "repo-mapping.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// --- ruleMatches tests ------------------------------------------------------

func TestRuleMatches_EmptyMatchMatchesAll(t *testing.T) {
	issue := makeIssue()
	if !ruleMatches(MappingMatch{}, issue) {
		t.Error("empty match should match every issue")
	}
}

func TestRuleMatches_Component(t *testing.T) {
	m := MappingMatch{Component: "flow"}
	if ruleMatches(m, makeIssue()) {
		t.Error("should not match issue with no components")
	}
	if !ruleMatches(m, makeIssue(withComponent("Flow Foundation"))) {
		t.Error("should match when component contains 'flow' (case-insensitive)")
	}
	if !ruleMatches(m, makeIssue(withComponent("Flow"))) {
		t.Error("should match exact component value")
	}
	if ruleMatches(m, makeIssue(withComponent("Banking"))) {
		t.Error("should not match unrelated component")
	}
}

func TestRuleMatches_MaintComponent(t *testing.T) {
	m := MappingMatch{MaintComponent: "Flow"}
	issue := makeIssue(withExtraField("Maint Component", "Flow"))
	if !ruleMatches(m, issue) {
		t.Error("should match when Maint Component matches")
	}
	if ruleMatches(m, makeIssue()) {
		t.Error("should not match issue without Maint Component field")
	}
	// Case-insensitive substring
	if !ruleMatches(MappingMatch{MaintComponent: "flow"}, issue) {
		t.Error("should match case-insensitively")
	}
}

func TestRuleMatches_SubComponent(t *testing.T) {
	m := MappingMatch{SubComponent: "Mobile"}
	issue := makeIssue(withExtraField("Maint Sub-component", "Mobile"))
	if !ruleMatches(m, issue) {
		t.Error("should match when Maint Sub-component matches")
	}
	if ruleMatches(m, makeIssue()) {
		t.Error("should not match issue without the sub-component field")
	}
	if !ruleMatches(MappingMatch{SubComponent: "mobile"}, issue) {
		t.Error("should match case-insensitively")
	}
}

func TestRuleMatches_Labels_AllRequired(t *testing.T) {
	m := MappingMatch{Labels: []string{"android", "critical"}}
	all := makeIssue(withLabel("android"), withLabel("critical"))
	partial := makeIssue(withLabel("android"))
	none := makeIssue()

	if !ruleMatches(m, all) {
		t.Error("should match when all required labels are present")
	}
	if ruleMatches(m, partial) {
		t.Error("should not match when only some labels are present")
	}
	if ruleMatches(m, none) {
		t.Error("should not match issue with no labels")
	}
}

func TestRuleMatches_Labels_CaseInsensitive(t *testing.T) {
	m := MappingMatch{Labels: []string{"Android"}}
	if !ruleMatches(m, makeIssue(withLabel("android"))) {
		t.Error("label match should be case-insensitive")
	}
}

func TestRuleMatches_SummaryContains(t *testing.T) {
	m := MappingMatch{SummaryContains: "[Android]"}
	if !ruleMatches(m, makeIssue(withSummary("[Android][Onboarding] crash"))) {
		t.Error("should match when summary contains the substring")
	}
	if ruleMatches(m, makeIssue(withSummary("[iOS] same crash"))) {
		t.Error("should not match when summary does not contain the substring")
	}
	if !ruleMatches(m, makeIssue(withSummary("[android] crash"))) {
		t.Error("summary match should be case-insensitive")
	}
}

func TestRuleMatches_AffectedVersion(t *testing.T) {
	m := MappingMatch{AffectedVersion: "2025.09"}
	if !ruleMatches(m, makeIssue(withAffectedVersion("2025.09-LTS"))) {
		t.Error("should match when an affected version contains the substring")
	}
	if ruleMatches(m, makeIssue(withAffectedVersion("2024.03-LTS"))) {
		t.Error("should not match when no affected version matches")
	}
	if ruleMatches(m, makeIssue()) {
		t.Error("should not match issue with no affected versions")
	}
}

func TestRuleMatches_AND_Logic(t *testing.T) {
	m := MappingMatch{
		MaintComponent: "Flow",
		SubComponent:   "Mobile",
		SummaryContains: "[Android]",
	}

	// All three satisfied.
	all := makeIssue(
		withSummary("[Android] crash in onboarding"),
		withExtraField("Maint Component", "Flow"),
		withExtraField("Maint Sub-component", "Mobile"),
	)
	if !ruleMatches(m, all) {
		t.Error("should match when all criteria are satisfied")
	}

	// Only two satisfied.
	partial := makeIssue(
		withSummary("[Android] crash"),
		withExtraField("Maint Component", "Flow"),
		// SubComponent missing
	)
	if ruleMatches(m, partial) {
		t.Error("should not match when one criterion is missing")
	}
}

// --- Resolve static rule tests -----------------------------------------------

func TestResolve_StaticRuleWins(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules: []MappingRule{
			{
				Description: "Android Flow Mobile",
				Match:       MappingMatch{MaintComponent: "Flow", SubComponent: "Mobile"},
				Tags:        []string{"android", "flow"},
			},
		},
	}
	path := writeMappingConfig(t, tmp, cfg)

	resolver, err := New(Options{MappingPath: path})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	issue := makeIssue(
		withExtraField("Maint Component", "Flow"),
		withExtraField("Maint Sub-component", "Mobile"),
	)
	res, err := resolver.Resolve(context.Background(), issue)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if res.Source != SourceStaticRule {
		t.Errorf("Source = %q, want %q", res.Source, SourceStaticRule)
	}
	if len(res.Tags) != 2 || res.Tags[0] != "android" || res.Tags[1] != "flow" {
		t.Errorf("Tags = %v, want [android flow]", res.Tags)
	}
	if res.Confidence != 1.0 {
		t.Errorf("Confidence = %g, want 1.0", res.Confidence)
	}
	if res.RuleDescription != "Android Flow Mobile" {
		t.Errorf("RuleDescription = %q, want %q", res.RuleDescription, "Android Flow Mobile")
	}
}

func TestResolve_FirstRuleWins(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules: []MappingRule{
			{
				Description: "first",
				Match:       MappingMatch{MaintComponent: "Flow"},
				Tags:        []string{"flow"},
			},
			{
				Description: "second",
				Match:       MappingMatch{MaintComponent: "Flow"},
				Tags:        []string{"flow", "extra"},
			},
		},
	}
	path := writeMappingConfig(t, tmp, cfg)
	resolver, _ := New(Options{MappingPath: path})

	issue := makeIssue(withExtraField("Maint Component", "Flow"))
	res, _ := resolver.Resolve(context.Background(), issue)
	if res.RuleDescription != "first" {
		t.Errorf("RuleDescription = %q, want %q", res.RuleDescription, "first")
	}
	if len(res.Tags) != 1 || res.Tags[0] != "flow" {
		t.Errorf("Tags = %v, want [flow]", res.Tags)
	}
}

func TestResolve_DefaultTagsWhenNoRuleMatches(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules: []MappingRule{
			{Match: MappingMatch{MaintComponent: "Flow"}, Tags: []string{"flow"}},
		},
		DefaultTags: []string{"backend"},
	}
	path := writeMappingConfig(t, tmp, cfg)
	resolver, _ := New(Options{MappingPath: path})

	issue := makeIssue(withExtraField("Maint Component", "Something Else"))
	res, err := resolver.Resolve(context.Background(), issue)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if res.Source != SourceDefault {
		t.Errorf("Source = %q, want %q", res.Source, SourceDefault)
	}
	if len(res.Tags) != 1 || res.Tags[0] != "backend" {
		t.Errorf("Tags = %v, want [backend]", res.Tags)
	}
	if res.Confidence != 0.3 {
		t.Errorf("Confidence = %g, want 0.3", res.Confidence)
	}
}

func TestResolve_EmptyWhenNoMatchAndNoDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules: []MappingRule{
			{Match: MappingMatch{MaintComponent: "Flow"}, Tags: []string{"flow"}},
		},
	}
	path := writeMappingConfig(t, tmp, cfg)
	resolver, _ := New(Options{MappingPath: path})

	issue := makeIssue() // no matching fields
	res, err := resolver.Resolve(context.Background(), issue)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !res.Empty() {
		t.Errorf("expected empty resolution, got %+v", res)
	}
}

func TestResolve_EmptyConfigAlwaysEmpty(t *testing.T) {
	// No config file: loadMappingConfig returns empty config without error.
	resolver, err := New(Options{MappingPath: filepath.Join(t.TempDir(), "repo-mapping.json")})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	res, err := resolver.Resolve(context.Background(), makeIssue())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !res.Empty() {
		t.Errorf("expected empty resolution when config has no rules, got %+v", res)
	}
}

func TestResolve_AIFallbackSkippedWithoutAPIKey(t *testing.T) {
	// AIFallback=true but no APIKey: should not panic or call the agent.
	resolver, _ := New(Options{
		MappingPath: filepath.Join(t.TempDir(), "repo-mapping.json"),
		AIFallback:  true,
		AIAPIKey:    "", // no key
	})
	res, err := resolver.Resolve(context.Background(), makeIssue())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !res.Empty() {
		t.Errorf("expected empty resolution when API key is missing, got %+v", res)
	}
}

func TestResolve_RuleDescriptionFallbackToKey(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules: []MappingRule{
			{
				// No Description set
				Match: MappingMatch{MaintComponent: "Flow"},
				Tags:  []string{"flow"},
			},
		},
	}
	path := writeMappingConfig(t, tmp, cfg)
	resolver, _ := New(Options{MappingPath: path})

	issue := makeIssue(withExtraField("Maint Component", "Flow"))
	issue.Key = "MAINT-99"
	res, _ := resolver.Resolve(context.Background(), issue)
	if res.RuleDescription == "" {
		t.Error("RuleDescription should not be empty when rule has no description")
	}
}

// --- loadMappingConfig tests -------------------------------------------------

func TestLoadMappingConfig_FileNotExistReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := loadMappingConfig(path)
	if err != nil {
		t.Fatalf("loadMappingConfig() error = %v, want nil for missing file", err)
	}
	if len(cfg.Rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(cfg.Rules))
	}
}

func TestLoadMappingConfig_ExplicitPathUsed(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules: []MappingRule{
			{Match: MappingMatch{MaintComponent: "Flow"}, Tags: []string{"flow"}},
		},
	}
	path := writeMappingConfig(t, tmp, cfg)
	loaded, err := loadMappingConfig(path)
	if err != nil {
		t.Fatalf("loadMappingConfig() error = %v", err)
	}
	if len(loaded.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(loaded.Rules))
	}
	if loaded.Rules[0].Tags[0] != "flow" {
		t.Errorf("Tags[0] = %q, want %q", loaded.Rules[0].Tags[0], "flow")
	}
}

func TestLoadMappingConfig_TriageHomeUsed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRIAGE_HOME", tmp)

	cfg := MappingConfig{DefaultTags: []string{"from-triage-home"}}
	writeMappingConfig(t, tmp, cfg)

	loaded, err := loadMappingConfig("") // no explicit path
	if err != nil {
		t.Fatalf("loadMappingConfig() error = %v", err)
	}
	if len(loaded.DefaultTags) != 1 || loaded.DefaultTags[0] != "from-triage-home" {
		t.Errorf("DefaultTags = %v, want [from-triage-home]", loaded.DefaultTags)
	}
}

func TestLoadMappingConfig_MalformedJSONReturnsError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "repo-mapping.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadMappingConfig(path)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

// --- parseAIResponse tests ---------------------------------------------------

func TestParseAIResponse_BareJSON(t *testing.T) {
	raw := `{"tags":["android","flow"],"confidence":0.85,"reasoning":"matches android"}`
	resp, err := parseAIResponse(raw)
	if err != nil {
		t.Fatalf("parseAIResponse() error = %v", err)
	}
	if len(resp.Tags) != 2 || resp.Tags[0] != "android" || resp.Tags[1] != "flow" {
		t.Errorf("Tags = %v, want [android flow]", resp.Tags)
	}
	if resp.Confidence != 0.85 {
		t.Errorf("Confidence = %g, want 0.85", resp.Confidence)
	}
	if resp.Reasoning != "matches android" {
		t.Errorf("Reasoning = %q, want %q", resp.Reasoning, "matches android")
	}
}

func TestParseAIResponse_FencedJSON(t *testing.T) {
	raw := "Here is my answer:\n```json\n{\"tags\":[\"ios\"],\"confidence\":0.9,\"reasoning\":\"iOS ticket\"}\n```"
	resp, err := parseAIResponse(raw)
	if err != nil {
		t.Fatalf("parseAIResponse() error = %v", err)
	}
	if len(resp.Tags) != 1 || resp.Tags[0] != "ios" {
		t.Errorf("Tags = %v, want [ios]", resp.Tags)
	}
}

func TestParseAIResponse_EmptyReturnsError(t *testing.T) {
	_, err := parseAIResponse("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseAIResponse_NoTagsReturnsError(t *testing.T) {
	_, err := parseAIResponse(`{"tags":[],"confidence":0.5,"reasoning":"nothing"}`)
	if err == nil {
		t.Error("expected error when tags array is empty")
	}
}

func TestParseAIResponse_InvalidJSONReturnsError(t *testing.T) {
	_, err := parseAIResponse("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- Resolution helpers -----------------------------------------------------

func TestResolution_Empty(t *testing.T) {
	if !(Resolution{}).Empty() {
		t.Error("zero-value Resolution should be empty")
	}
	if (Resolution{Tags: []string{"a"}}).Empty() {
		t.Error("Resolution with tags should not be empty")
	}
}

// --- Config return test -----------------------------------------------------

func TestResolverConfig(t *testing.T) {
	tmp := t.TempDir()
	cfg := MappingConfig{
		Rules:       []MappingRule{{Tags: []string{"x"}}},
		DefaultTags: []string{"fallback"},
	}
	path := writeMappingConfig(t, tmp, cfg)
	r, err := New(Options{MappingPath: path})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	loaded := r.Config()
	if len(loaded.Rules) != 1 {
		t.Errorf("Config().Rules len = %d, want 1", len(loaded.Rules))
	}
	if loaded.DefaultTags[0] != "fallback" {
		t.Errorf("Config().DefaultTags[0] = %q, want %q", loaded.DefaultTags[0], "fallback")
	}
}
