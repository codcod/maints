package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/codcod/maints-triage/internal/config"
	"github.com/codcod/maints-triage/internal/dash"
	"github.com/codcod/maints-triage/internal/dig"
	"github.com/codcod/maints-triage/internal/jira"
	"github.com/codcod/maints-triage/internal/open"
	"github.com/codcod/maints-triage/internal/release"
	"github.com/codcod/maints-triage/internal/schedule"
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
	root := &cobra.Command{
		Use:     "maints",
		Short:   "CLI for maintenance workflows",
		Version: version,
		Long: `maints groups commands for working with Jira maintenance issues and related tooling.

Use "maints <command> --help" for details on a specific command.`,
	}
	root.AddCommand(newTriageCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newDigCmd())
	root.AddCommand(newDashCmd())
	root.AddCommand(newOpenCmd())
	root.AddCommand(newScheduleCmd())
	root.AddCommand(newReleaseCmd())
	return root
}

func newOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open [ISSUE-KEY]...",
		Short: "Open Jira issues in the default web browser",
		Long: `open uses $JIRA_URL to build a browse link for each key and opens
it in your default browser. Only JIRA_URL is required; no Jira API credentials are needed.

Issue keys look like PROJ-123 (letters, numbers, and underscores in the project part).`,
		Example: `  maints open MAINT-41509
  maints open MAINT-1 DIG-2`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := config.LoadJiraURLOnly()
			if err != nil {
				return err
			}
			for _, a := range args {
				k, err := open.ValidateKey(a)
				if err != nil {
					return err
				}
				u := open.IssueBrowseURL(base, k)
				if err := open.OpenBrowser(u); err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			}
			return nil
		},
	}
	return cmd
}

func newDashCmd() *cobra.Command {
	var (
		jql        string
		digProject string
		linkType   string
		assignee   string
		supervisor bool
		columns    string
		debug      bool
		noDig      bool
	)
	cmd := &cobra.Command{
		Use:   "dash",
		Short: "Print a terminal dashboard of your MAINT Flow issues and linked DIG work",
		Long: `dash runs a fixed JQL (overridable with --jql) to list your MAINT Flow
tickets, then for each one shows linked DIG issues connected with the same link
type used by "maints dig" (default "Solved by", or $JIRA_LINK_TYPE).

You can also target another assignee with --assignee (same built-in JQL, but
assignee = the given string). Use --supervisor for the same built-in filter
without an assignee restriction (overview across assignees). Do not combine
--jql with --assignee or --supervisor.

Use --no-dig to print only MAINT rows (no linked DIG lines or DIG API calls).

Requires Jira credentials only (no cursor-agent).`,
		Example: `  maints dash
  maints dash --no-dig
  maints dash --assignee colleague@example.com
  maints dash --supervisor
  maints dash --columns "key, priority, due"
  maints dash --columns "key, summary[20], scheduled, assignee"
  maints dash --dig-project DIG
  maints dash --jql 'project = MAINT AND assignee = currentUser() ORDER BY created ASC'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadJiraOnly()
			if err != nil {
				return err
			}
			client := jira.NewClient(cfg.JiraURL, cfg.JiraUsername, cfg.JiraAPIToken)
			return dash.Run(cmd.Context(), client, cmd.OutOrStdout(), cmd.ErrOrStderr(), dash.Options{
				JQL:        jql,
				DigProject: digProject,
				LinkType:   linkType,
				Assignee:   assignee,
				Supervisor: supervisor,
				Columns:    columns,
				Debug:      debug,
				NoDig:      noDig,
			})
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "override the default JQL (see docs/dash.md for the built-in query; mutually exclusive with --assignee and --supervisor)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "built-in JQL: filter assignee to this Jira user (email, name, or id; mutually exclusive with --jql and --supervisor)")
	cmd.Flags().BoolVar(&supervisor, "supervisor", false, "built-in JQL: all Flow MAINT assignees (no assignee filter; mutually exclusive with --jql and --assignee)")
	cmd.Flags().StringVar(&digProject, "dig-project", "DIG", "Jira project key for linked work items (e.g. DIG)")
	cmd.Flags().StringVar(&linkType, "link-type", "", `issue link name to follow (default: $JIRA_LINK_TYPE or "Solved by")`)
	cmd.Flags().StringVar(&columns, "columns", "", `comma-separated column names: key, priority, status, due, summary, scheduled, assignee (default: all, in that order; case-insensitive). Use summary[N] for a custom max width in runes (e.g. summary[20]); default is 50`)
	cmd.Flags().BoolVar(&debug, "debug", false, "print each issue's issuelinks (type names, keys) to stderr")
	cmd.Flags().BoolVar(&noDig, "no-dig", false, "print only MAINT rows (no linked DIG sub-rows or DIG fetches)")
	return cmd
}

func newDigCmd() *cobra.Command {
	var (
		query        string
		digProject   string
		issueType    string
		linkType     string
		linkSwapEnds bool
	)

	cmd := &cobra.Command{
		Use:   "dig [ISSUE-KEY]...",
		Short: "Duplicate Jira issues into DIG (or another project) with team and issue link",
		Long: `dig mirrors each source issue into the target project (default DIG): same summary,
a duplicate notice with the source issue key linked to the MAINT browse URL, the team
from JIRA_TEAM_ID on JIRA_TEAM_FIELD, the same assignee when present, and an issue link
(default type "Solved by", overridable via flags or JIRA_LINK_TYPE).

Required environment variables (no defaults):
  JIRA_URL           Jira base URL (https://…)
  JIRA_USERNAME      Account email
  JIRA_API_TOKEN     API token
  JIRA_TEAM_FIELD    Custom field id for Team (e.g. customfield_14700)
  JIRA_TEAM_ID       Atlassian team UUID

Optional environment variables:
  JIRA_DIG_ISSUE_TYPE    Default issue type in DIG when --issue-type is omitted (default Bug)
  JIRA_LINK_TYPE         Default link type name when --link-type is omitted
  JIRA_SOLVES_LINK_TYPE  Deprecated: used if JIRA_LINK_TYPE is unset`,
		Example: `  maints dig MAINT-1 MAINT-2
  maints dig --query 'project = MAINT AND status = Open'
  maints dig --issue-type Story MAINT-1
  maints dig --link-swap-ends MAINT-1
  maints dig --dig-project DIG --link-type "Solved by" MAINT-1`,
		Args: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(query) != "" && len(args) > 0 {
				return fmt.Errorf("use either issue keys or --query, not both")
			}
			if strings.TrimSpace(query) == "" && len(args) == 0 {
				return fmt.Errorf("provide issue keys or --query JQL")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadJiraOnly()
			if err != nil {
				return err
			}

			teamField := strings.TrimSpace(os.Getenv("JIRA_TEAM_FIELD"))
			teamID := strings.TrimSpace(os.Getenv("JIRA_TEAM_ID"))

			it := strings.TrimSpace(issueType)
			if it == "" {
				it = dig.DefaultIssueType()
			}
			lt := strings.TrimSpace(linkType)
			if lt == "" {
				lt = dig.DefaultLinkType()
			}

			client := jira.NewClient(cfg.JiraURL, cfg.JiraUsername, cfg.JiraAPIToken)
			return dig.Run(cmd.Context(), client, dig.Options{
				Keys:         args,
				JQL:          strings.TrimSpace(query),
				DigProject:   digProject,
				IssueType:    it,
				LinkType:     lt,
				LinkSwapEnds: linkSwapEnds,
				TeamField:    teamField,
				TeamID:       teamID,
			}, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "JQL to select source issues (mutually exclusive with issue keys)")
	cmd.Flags().StringVar(&digProject, "dig-project", "DIG", "target Jira project key for new issues")
	cmd.Flags().StringVar(&issueType, "issue-type", "", "issue type name in target project (default: $JIRA_DIG_ISSUE_TYPE or Bug)")
	cmd.Flags().StringVar(&linkType, "link-type", "", `issue link type name (default: $JIRA_LINK_TYPE, $JIRA_SOLVES_LINK_TYPE, or "Solved by")`)
	cmd.Flags().BoolVar(&linkSwapEnds, "link-swap-ends", false, "swap outward/inward ends for the issue link")
	return cmd
}

func newScheduleCmd() *cobra.Command {
	var (
		query        string
		versions     []string
		remove       bool
		linkType     string
		digProject   string
		maintProject string
	)
	cmd := &cobra.Command{
		Use:   "schedule [DIG-KEY]...",
		Short: "Set or remove fix version(s) on DIG issues; comment on linked MAINTs when Jira update succeeds",
		Long: `schedule sets fixVersions on each DIG (adds names, or removes them with --remove)
using the same issue link as maints dig / maints dash (default "Solved by", or $JIRA_LINK_TYPE)
to find the linked MAINT. After a successful update, it posts a Patch Releases comment
on that MAINT for each version that was added or removed.

Requires: JIRA_URL, JIRA_USERNAME, JIRA_API_TOKEN.

Example:
  maints schedule DIG-1 DIG-2 --version "DS 2025.09.5" --version "1.0"
  maints schedule --query "project = DIG and status = 'In Progress'" --version "1.0"
  maints schedule DIG-1 --version "1.0" --remove`,
		Args: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(query) != "" && len(args) > 0 {
				return fmt.Errorf("use either issue keys or --query, not both")
			}
			if strings.TrimSpace(query) == "" && len(args) == 0 {
				return fmt.Errorf("provide DIG key(s) or --query JQL")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadJiraOnly()
			if err != nil {
				return err
			}
			lt := strings.TrimSpace(linkType)
			if lt == "" {
				lt = dig.DefaultLinkType()
			}
			client := jira.NewClient(cfg.JiraURL, cfg.JiraUsername, cfg.JiraAPIToken)
			return schedule.Run(cmd.Context(), client, schedule.Options{
				Keys:         args,
				JQL:          strings.TrimSpace(query),
				Versions:     versions,
				Remove:       remove,
				LinkType:     lt,
				DigProject:   digProject,
				MaintProject: maintProject,
			}, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringArrayVar(&versions, "version", nil, "fix version name to set or remove (required; use multiple times for several versions)")
	cmd.Flags().StringVar(&query, "query", "", "JQL to select DIG issues (mutually exclusive with issue keys on the command line)")
	cmd.Flags().BoolVar(&remove, "remove", false, "remove the given fix version name(s) instead of adding them")
	cmd.Flags().StringVar(&linkType, "link-type", "", `link type to follow to MAINT (default: $JIRA_LINK_TYPE or "Solved by")`)
	cmd.Flags().StringVar(&digProject, "dig-project", "DIG", "project key to validate command-line and query results (e.g. DIG)")
	cmd.Flags().StringVar(&maintProject, "maint-project", "MAINT", "key prefix of MAINT to comment on (e.g. MAINT)")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newReleaseCmd() *cobra.Command {
	var (
		digProject   string
		maintProject string
		linkType     string
		supervisor   bool
	)
	cmd := &cobra.Command{
		Use:   "release [FIX_VERSION]",
		Short: "After a patch release, update linked MAINTs (comments; close when all DIGs are done)",
		Long: `release finds all DIG issues with the given fix version (project = DIG and fixVersion = the argument).
It first checks Jira: the fix version must exist on the DIG project and be marked Released (not only unreleased).

For each DIG that is Done or Closed, it looks up the linked MAINT (same "Solved by" link as maints dig and maints dash)
and for each such MAINT (once per key):

- If every DIG linked to that MAINT with the same link type is Done or Closed, the MAINT is
  transitioned to status Done (resolution "Done" when the workflow allows it) and a comment
  is added: "All patches have been released now, closing."
  By default, closing is only performed when the MAINT is assigned to the authenticated Jira user
  (the same as JQL currentUser()). Use --supervisor to allow closing MAINTs with any assignee.
- If any of those DIGs are still not Done/Closed, a comment is added:
  "Patch <fix version> has been released, keeping MAINT open until all patches are released."

DIG issues in the fix version that are not Done or Closed produce a warning and are skipped for
MAINT follow-up (so their MAINTs are not considered from that DIG).

Requires: JIRA_URL, JIRA_USERNAME, JIRA_API_TOKEN.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadJiraOnly()
			if err != nil {
				return err
			}
			lt := strings.TrimSpace(linkType)
			if lt == "" {
				lt = dig.DefaultLinkType()
			}
			client := jira.NewClient(cfg.JiraURL, cfg.JiraUsername, cfg.JiraAPIToken)
			return release.Run(cmd.Context(), client, release.Options{
				FixVersion:   args[0],
				DigProject:   digProject,
				MaintProject: maintProject,
				LinkType:     lt,
				Supervisor:   supervisor,
			}, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&linkType, "link-type", "", `issue link to MAINT (default: $JIRA_LINK_TYPE or "Solved by")`)
	cmd.Flags().StringVar(&digProject, "dig-project", "DIG", "Jira project key for DIG (fix version scope)")
	cmd.Flags().StringVar(&maintProject, "maint-project", "MAINT", "key prefix of MAINT to update")
	cmd.Flags().BoolVar(&supervisor, "supervisor", false, "allow closing MAINTs not assigned to the current user (default: only close your MAINTs)")
	return cmd
}

func newTriageCmd() *cobra.Command {
	var (
		checklistPath string
		promptPath    string
		model         string
		concurrency   int
	)

	cmd := &cobra.Command{
		Use:   "triage <ISSUE-KEY> [ISSUE-KEY...]",
		Short: "Triage Jira maintenance issues using cursor-agent",
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
  MAINTS_HOME      Directory for maints configuration files
                   (default: $XDG_CONFIG_HOME/maints, or ~/.config/maints)`,
		Example: `  maints triage PROJ-123
  maints triage PROJ-123 PROJ-456
  maints triage --checklist ./custom-checklist.md PROJ-123
  maints triage --prompt ./custom-prompt.md PROJ-123
  maints triage --model sonnet-4 PROJ-123
  maints triage --concurrency 3 PROJ-123 PROJ-456 PROJ-789`,
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
		Example: `  maints serve --project MAINT
  maints serve --project MAINT --interval 10m
  maints serve --project MAINT --statuses "Open,Triage,Backlog"
  maints serve --jql 'project = Maintenance AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow) AND status IN (Open, Triage)'`,
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
