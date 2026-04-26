# Serve command (`maints serve`)

Start a server that periodically polls Jira and triages new issues automatically.

## Configuration

`maints serve` requires the **Global Jira Settings** and **Agent Settings** (`CURSOR_API_KEY`). See the [main README](../README.md#configuration) for details.

The file layout under `~/.config/maints/` (checklist, prompt, etc.) applies exactly the same as for `maints triage`; see the [Triage documentation](triage.md#configuration-directory) for details.

## Usage

Run as a daemon to poll Jira and triage new issues that match your criteria:

```bash
maints serve --project MAINT
```

Customize the polling interval and the statuses to watch:

```bash
maints serve --project MAINT --interval 10m --statuses "Open,Triage,Backlog"
```

Or use a raw JQL query (takes precedence over `--project` and `--statuses`):

```bash
maints serve --jql 'project = Maintenance AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow) AND status IN (Open, Triage)'
```

An issue is considered already triaged when its output directory exists under `triaged-maints/<KEY>/`. No separate state file is maintained.

In a future release, this command will also post the triage outcome as a Jira comment and transition the ticket to the appropriate workflow status.

## Flags

- `-c, --checklist`: Path to the checklist Markdown file (default: `./checklist.md`).
- `-j, --concurrency`: Max issues to triage in parallel (default `5`).
- `-i, --interval`: How often to poll Jira for new issues (default `5m0s`).
- `--jql`: Raw JQL query to find issues.
- `--model`: `cursor-agent` model to use (e.g., `sonnet-4`, `gpt-5`).
- `-P, --project`: Jira project key to poll when `--jql` is not set.
- `-p, --prompt`: Path to the prompt template Markdown file (default: `./triage-prompt.md`).
- `--statuses`: Comma-separated Jira statuses to watch when `--jql` is not set (default `Open,Triage`).
