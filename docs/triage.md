# Triage command (`maints triage`)

Fetch Jira issues, run checklist evaluation via `cursor-agent`, and write reports under `triaged-maints/<KEY>/`.

## Configuration

### Jira (required)

`maints triage` reads issues from Jira. Set:

| Variable | Description |
| -------- | ----------- |
| `JIRA_URL` | Base URL (e.g. `https://your-company.atlassian.net`) |
| `JIRA_USERNAME` | Jira account email |
| `JIRA_API_TOKEN` | [API token](https://id.atlassian.com/manage-profile/security/api-tokens) |

You can set these in the environment or in a `.env` file in the working directory.

### Cursor agent (required)

Triage runs **`cursor-agent`**: install it and ensure it is on your `PATH`. The tool passes through your API key; create one in the [Cursor dashboard](https://cursor.com/dashboard?tab=integrations).

| Variable | Description |
| -------- | ----------- |
| `CURSOR_API_KEY` | Required for the agent; forwarded to `cursor-agent` as `CURSOR_API_KEY` |

### Configuration directory

Files (checklist, prompt, field mappings, optional knowledge base index) are resolved in this order:

1. Explicit command-line flags (where available)
2. `$MAINTS_HOME/<filename>` if that path exists
3. `$XDG_CONFIG_HOME/maints/<filename>` (defaults to `~/.config/maints/<filename>`)
4. `./<filename>` in the current directory

Override the XDG default with an absolute path (environment or `.env`):

| Variable | Description |
| -------- | ----------- |
| `MAINTS_HOME` | Optional. If set, used for step 2 above instead of `$XDG_CONFIG_HOME/maints` |

Placing files under `MAINTS_HOME` (or the default `~/.config/maints/`) is optional. Typical files:

| File | Role |
| ---- | ---- |
| `checklist.md` | Checklist the agent uses for each issue |
| `triage-prompt.md` | Prompt template for the agent (see `maints triage --help` for the default when missing) |
| `fields-mapping.json` | Optional extra Jira field extraction (see below) |
| `kb-index.md` | Optional knowledge base index, if present in the maints config directory |

## Basic usage

Triage a single issue:

```bash
maints triage MAINT-123
```

Triage multiple issues in one go:

```bash
maints triage MAINT-123 MAINT-456 MAINT-789
```

## Custom checklist

To use an explicit checklist (overriding default locations):

```bash
maints triage --checklist ./my-custom-checklist.md MAINT-123
```

## Custom prompt

To use a custom prompt template (overriding default locations):

```bash
maints triage --prompt ./my-custom-prompt.md MAINT-123
```

## AI model selection

Specify which AI model the agent should use (e.g., `sonnet-4`, `gpt-4o`):

```bash
maints triage --model sonnet-4 MAINT-123
```

## Output format

Machine-readable JSON (one object per issue) is written to stdout, which is convenient for piping:

```bash
maints triage MAINT-123 | jq .
```

## Concurrent batch triage

By default, up to **5** issues are triaged in parallel. Each triage involves a
Jira HTTP round-trip and a `cursor-agent` invocation, so concurrency cuts wall
time roughly proportionally. Use `--concurrency` / `-j` to tune the limit:

```bash
# triage 10 issues with 3 parallel workers
maints triage --concurrency 3 MAINT-100 MAINT-101 MAINT-102 MAINT-103 MAINT-104 \
                             MAINT-105 MAINT-106 MAINT-107 MAINT-108 MAINT-109

# force sequential execution
maints triage --concurrency 1 MAINT-100 MAINT-101
```

Results are always printed in the order the issue keys were supplied, regardless
of completion order.

## Custom field mappings

You can extract additional fields from Jira issues by creating a
`fields-mapping.json` file in your maints configuration directory (see
[Configuration directory](#configuration-directory) above).

The file should contain a JSON array of mappings, where each mapping has a
`field` (display name) and a `path` (dot-notation path to the value in the Jira
JSON response).

Example `fields-mapping.json`:

```json
[
  {
    "field": "Customer Impact",
    "path": "fields.customfield_12345.value"
  },
  {
    "field": "Root Cause",
    "path": "fields.customfield_67890"
  }
]
```

## How it works

1. **Fetch**: Connects to the Jira REST API to retrieve the issue summary,
   description, comments, and metadata (status, priority, versions, components,
   etc.).
2. **Prepare**: Creates a `triaged-maints/KEY/` directory in the current
   working directory and writes two files into it:
   - `issue-KEY.md`: A Markdown-formatted representation of the Jira issue.
   - `checklist.md`: The triage checklist.
3. **Analyze**: Invokes `cursor-agent` in headless mode with
   `triaged-maints/KEY/` as its workspace. The agent is instructed via the
   prompt template (e.g., `triage-prompt.md`) to read both files and evaluate
   the issue against each checklist item.
4. **Report**: The agent's response is printed to stdout and also saved as
   `triaged-maints/KEY/report-KEY.md`. A machine-readable JSON summary is saved
   to `triaged-maints/KEY/report-KEY.json`.

After a run the directory is kept so you can review or commit the artefacts:

```
triaged-maints/
└── MAINT-123/
    ├── issue-MAINT-123.md
    ├── checklist.md
    ├── report-MAINT-123.md
    └── report-MAINT-123.json
```
