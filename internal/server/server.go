package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codcod/maints-triage/internal/config"
	"github.com/codcod/maints-triage/internal/jira"
	"github.com/codcod/maints-triage/internal/triage"
)

const (
	defaultInterval   = 5 * time.Minute
	defaultOutputDir  = "triaged-maints"
	defaultMaxResults = 50
)

// Options controls the behaviour of the auto-triage server.
type Options struct {
	// JQL is a raw Jira Query Language string used to find issues on every poll.
	// When set it takes precedence over Project and Statuses, which are ignored.
	// Example:
	//   project = Maintenance
	//   AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow)
	//   AND status IN (Open, Triage)
	JQL string

	// Project is the Jira project key used to build the default JQL when JQL
	// is empty (e.g. "MAINT"). Required when JQL is not set.
	Project string

	// Statuses is the list of Jira status names used to build the default JQL
	// when JQL is empty. Defaults to ["Open", "Triage"] when empty.
	Statuses []string

	// Interval is how often to poll Jira for new issues.
	// Defaults to 5 minutes when zero.
	Interval time.Duration

	// OutputDir is the root directory where triage output is written.
	// Defaults to "triaged-maints" when empty.
	OutputDir string

	// MaxResults caps the number of issues returned per Jira search call.
	// Defaults to 50 when zero.
	MaxResults int

	// TriageOptions are forwarded as-is to triage.Run for each batch.
	TriageOptions triage.Options
}

func (o *Options) withDefaults() Options {
	out := *o
	if out.JQL == "" && len(out.Statuses) == 0 {
		out.Statuses = []string{"Open", "Triage"}
	}
	if out.Interval <= 0 {
		out.Interval = defaultInterval
	}
	if out.OutputDir == "" {
		out.OutputDir = defaultOutputDir
	}
	if out.MaxResults <= 0 {
		out.MaxResults = defaultMaxResults
	}
	return out
}

// effectiveJQL returns the JQL string to use: the explicit JQL when set,
// otherwise one built from Project and Statuses.
func (o Options) effectiveJQL() string {
	if o.JQL != "" {
		return o.JQL
	}
	quotedStatuses := make([]string, len(o.Statuses))
	for i, s := range o.Statuses {
		quotedStatuses[i] = `"` + s + `"`
	}
	return fmt.Sprintf(`project = "%s" AND status in (%s) ORDER BY created DESC`,
		o.Project, strings.Join(quotedStatuses, ","))
}

// Run starts the auto-triage polling loop and blocks until ctx is cancelled.
//
// On every tick it:
//  1. Searches Jira for issues in the configured project + statuses.
//  2. Filters out issues that already have an output directory (already triaged).
//  3. Runs triage.Run on the remaining keys.
//
// Progress and errors are written as human-readable lines to w.
func Run(ctx context.Context, cfg *config.Config, opts Options, w io.Writer) error {
	opts = opts.withDefaults()

	jiraClient := jira.NewClient(cfg.JiraURL, cfg.JiraUsername, cfg.JiraAPIToken)

	// Route per-issue triage progress into the server's log writer.
	opts.TriageOptions.Log = w

	logf(w, "server started: jql=%q interval=%s", opts.effectiveJQL(), opts.Interval)

	// Run immediately on start, then on every tick.
	if err := poll(ctx, jiraClient, cfg, opts, w); err != nil {
		logf(w, "poll error: %s", err)
	}

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logf(w, "server stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := poll(ctx, jiraClient, cfg, opts, w); err != nil {
				logf(w, "poll error: %s", err)
			}
		}
	}
}

// poll performs a single search-and-triage cycle.
func poll(ctx context.Context, jiraClient *jira.Client, cfg *config.Config, opts Options, w io.Writer) error {
	jql := opts.effectiveJQL()
	logf(w, "polling Jira: %s", jql)

	keys, err := jiraClient.SearchIssues(ctx, jql, opts.MaxResults)
	if err != nil {
		return fmt.Errorf("search issues: %w", err)
	}
	logf(w, "found %d issue(s) in Jira", len(keys))

	newKeys := filterUntriaged(keys, opts.OutputDir)
	if len(newKeys) == 0 {
		logf(w, "no new issues to triage")
		return nil
	}
	logf(w, "triaging %d new issue(s): %v", len(newKeys), newKeys)

	// Discard the JSON batch written by triage.Run — in server mode the reports
	// are already saved to triaged-maints/<KEY>/report-<KEY>.md and per-issue
	// progress is streamed to w via TriageOptions.Log above.
	if err := triage.Run(ctx, newKeys, cfg, opts.TriageOptions, io.Discard); err != nil {
		return fmt.Errorf("triage run: %w", err)
	}
	logf(w, "triage complete for %d issue(s)", len(newKeys))
	return nil
}

// filterUntriaged returns only the keys that do not yet have an output
// directory under outputDir. Directory existence is treated as the canonical
// signal that an issue has already been triaged.
func filterUntriaged(keys []string, outputDir string) []string {
	var out []string
	for _, key := range keys {
		dir := filepath.Join(outputDir, key)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			out = append(out, key)
		}
	}
	return out
}

func logf(w io.Writer, format string, args ...any) {
	ts := time.Now().Format(time.RFC3339)
	_, _ = fmt.Fprintf(w, "%s  %s\n", ts, fmt.Sprintf(format, args...))
}
