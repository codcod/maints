package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/codcod/maints-triage/internal/config"
	"github.com/codcod/maints-triage/internal/server"
	"github.com/codcod/maints-triage/internal/triage"
)

// version is set at build time via -ldflags="-X main.version=<tag>".
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		checklistPath string
		promptPath    string
		model         string
		concurrency   int
	)

	cmd := &cobra.Command{
		Use:     "triage <ISSUE-KEY> [ISSUE-KEY...]",
		Short:   "Triage Jira maintenance issues using cursor-agent",
		Version: version,
		Long: `triage fetches Jira maintenance issues and runs cursor-agent to verify
completeness against a configurable checklist.

Each issue is evaluated and a machine-readable JSON decision is printed to
stdout (one JSON object per issue). The full human-readable report is saved to
triaged-maints/<KEY>/report-<KEY>.md.

The decision field indicates the recommended Jira workflow transition:
  accept       — move ticket to IN ANALYSIS
  request_info — move ticket to AWAITING INPUT (questions listed)
  reject       — close the ticket (rejection reason stated)

Required environment variables (or .env file):
  JIRA_URL         Base URL of your Jira instance (e.g. https://acme.atlassian.net)
  JIRA_USERNAME    Jira account email
  JIRA_API_TOKEN   Jira API token
  CURSOR_API_KEY   cursor-agent API key

Optional environment variables:
  TRIAGE_HOME      Directory for triage configuration files
                   (default: $XDG_CONFIG_HOME/triage, or ~/.config/triage)`,
		Example: `  triage PROJ-123
  triage PROJ-123 PROJ-456
  triage --checklist ./custom-checklist.md PROJ-123
  triage --prompt ./custom-prompt.md PROJ-123
  triage --model sonnet-4 PROJ-123
  triage --concurrency 3 PROJ-123 PROJ-456 PROJ-789`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			return triage.Run(cmd.Context(), args, cfg, triage.Options{
				ChecklistPath: checklistPath,
				PromptPath:    promptPath,
				Model:         model,
				Concurrency:   concurrency,
			}, os.Stdout)
		},
	}

	cmd.Flags().StringVarP(&checklistPath, "checklist", "c", "",
		`path to the checklist Markdown file (default: "./checklist.md")`)
	cmd.Flags().StringVarP(&promptPath, "prompt", "p", "",
		`path to the prompt template Markdown file (default: "./triage-prompt.md")`)
	cmd.Flags().StringVar(&model, "model", "",
		"cursor-agent model to use (e.g. sonnet-4, gpt-5)")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 0,
		"max issues to triage in parallel (default 5)")

	cmd.AddCommand(newServeCmd())

	return cmd
}

func newServeCmd() *cobra.Command {
	var (
		jql           string
		project       string
		statusesRaw   string
		interval      time.Duration
		checklistPath string
		promptPath    string
		model         string
		concurrency   int
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a server that periodically triages new Jira issues",
		Long: `serve polls Jira at a configurable interval, finds issues matching the
configured query, and automatically triages any that have not already been processed.

Supply either --jql with a full JQL string, or --project (with optional --statuses)
to have the query built automatically. --jql takes precedence when both are given.

An issue is considered already triaged when its output directory exists under
triaged-maints/<KEY>/. No separate state file is maintained.

In a future release this command will also post the triage outcome as a Jira
comment and transition the ticket to the appropriate workflow status.`,
		Example: `  triage serve --project MAINT
  triage serve --project MAINT --interval 10m
  triage serve --project MAINT --statuses "Open,Triage,Backlog"
  triage serve --jql 'project = Maintenance AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow) AND status IN (Open, Triage)'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jql == "" && project == "" {
				return fmt.Errorf("either --jql or --project must be provided")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			var statuses []string
			for _, s := range strings.Split(statusesRaw, ",") {
				if t := strings.TrimSpace(s); t != "" {
					statuses = append(statuses, t)
				}
			}

			return server.Run(cmd.Context(), cfg, server.Options{
				JQL:        jql,
				Project:    project,
				Statuses:   statuses,
				Interval:   interval,
				MaxResults: 50,
				TriageOptions: triage.Options{
					ChecklistPath: checklistPath,
					PromptPath:    promptPath,
					Model:         model,
					Concurrency:   concurrency,
				},
			}, os.Stdout)
		},
	}

	cmd.Flags().StringVar(&jql, "jql", "",
		"raw JQL query to find issues (takes precedence over --project/--statuses when set)")
	cmd.Flags().StringVarP(&project, "project", "P", "",
		"Jira project key to poll when --jql is not set (e.g. MAINT)")
	cmd.Flags().StringVar(&statusesRaw, "statuses", "Open,Triage",
		"comma-separated Jira statuses to watch (used when --jql is not set)")
	cmd.Flags().DurationVarP(&interval, "interval", "i", 5*time.Minute,
		"how often to poll Jira for new issues")
	cmd.Flags().StringVarP(&checklistPath, "checklist", "c", "",
		`path to the checklist Markdown file (default: "./checklist.md")`)
	cmd.Flags().StringVarP(&promptPath, "prompt", "p", "",
		`path to the prompt template Markdown file (default: "./triage-prompt.md")`)
	cmd.Flags().StringVar(&model, "model", "",
		"cursor-agent model to use (e.g. sonnet-4, gpt-5)")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 0,
		"max issues to triage in parallel (default 5)")

	return cmd
}
