# Dig command (`maints dig`)

Duplicate Jira issues into DIG (or another project) with the team set and an
issue link created back to the source.

## Configuration

This command requires the **Global Jira Settings** and **Dig Settings**
(`JIRA_TEAM_FIELD`, `JIRA_TEAM_ID`). See the [main
README](../README.md#configuration) for details. It does **not** use
`cursor-agent` or `CURSOR_API_KEY`.

Command-line flags override the environment defaults for project, issue type,
and link type where supported.

## Usage

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

Swap the outward/inward ends of the issue link if it is created in the wrong
direction:

```bash
maints dig --link-swap-ends MAINT-1
```

## Flags

- `--dig-project`: Target Jira project key for new issues (default `DIG`).
- `--issue-type`: Issue type name in target project (default:
  `$JIRA_DIG_ISSUE_TYPE` or `Bug`).
- `--link-swap-ends`: Swap outward/inward ends for the issue link.
- `--link-type`: Issue link type name (default: `$JIRA_LINK_TYPE`,
  `$JIRA_SOLVES_LINK_TYPE`, or `Solved by`).
- `--query`: JQL to select source issues (mutually exclusive with issue keys).
