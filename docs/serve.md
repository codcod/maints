# Serve command (`maints serve`)

## Configuration

`maints serve` polls Jira, then triages new issues the same way as
[`maints triage`](triage.md) (same Jira connection, `cursor-agent`, and `CURSOR_API_KEY`).

| Variable | Description |
| -------- | ----------- |
| `JIRA_URL` | Jira base URL |
| `JIRA_USERNAME` | Jira account email |
| `JIRA_API_TOKEN` | Jira API token |
| `CURSOR_API_KEY` | `cursor-agent` API key (see [triage.md](triage.md#cursor-agent-required)) |

`MAINTS_HOME` and the file layout under `~/.config/maints/` (checklist, prompt, etc.) apply the same as for `maints triage`; see [Triage: configuration directory](triage.md#configuration-directory).

Run as a daemon to poll Jira and triage new issues that match your criteria:

```bash
maints serve --project MAINT
```

Customize the polling interval and the statuses to watch:

```bash
maints serve --project MAINT --interval 10m --statuses "Open,Triage,Backlog"
```

Or use a raw JQL query:

```bash
maints serve --jql 'project = Maintenance AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow) AND status IN (Open, Triage)'
```
