# Dig command (`maints dig`)

## Configuration

`maints dig` talks to Jira only: it does **not** use `cursor-agent` or `CURSOR_API_KEY`.

| Variable | Required | Description |
| -------- | -------- | ----------- |
| `JIRA_URL` | Yes | Base URL (e.g. `https://your-company.atlassian.net`) |
| `JIRA_USERNAME` | Yes | Jira account email |
| `JIRA_API_TOKEN` | Yes | [API token](https://id.atlassian.com/manage-profile/security/api-tokens) |
| `JIRA_TEAM_FIELD` | Yes | Custom field id for Team (e.g. `customfield_14700`) — no default |
| `JIRA_TEAM_ID` | Yes | Atlassian team UUID — no default |

| Variable | Required | Description |
| -------- | -------- | ----------- |
| `JIRA_DIG_ISSUE_TYPE` | No | Default target issue type when `--issue-type` is omitted (default: `Bug`) |
| `JIRA_LINK_TYPE` | No | Default link type when `--link-type` is omitted |
| `JIRA_SOLVES_LINK_TYPE` | No | Deprecated: used only if `JIRA_LINK_TYPE` is unset |

Set these via the environment or a `.env` file. Command-line flags override the defaults for project, issue type, and link type where supported.

## Usage

Duplicate Jira issues into DIG (or another project) with the team set and an issue link created back to the source.

Duplicate specific issues:

```bash
maints dig MAINT-1 MAINT-2
```

Duplicate issues matching a JQL query:

```bash
maints dig --query 'project = MAINT AND status = Open'
```

Customize the target project, issue type, or link type:

```bash
maints dig --dig-project DIG --issue-type Story --link-type "Solved by" MAINT-1
```

Swap the outward/inward ends of the issue link if it's created in the wrong direction:

```bash
maints dig --link-swap-ends MAINT-1
```
