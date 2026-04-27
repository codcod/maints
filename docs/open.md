# Open command (`maints open`)

Open one or more Jira issues in your default web browser. Each key becomes
`{JIRA_URL}/browse/KEY` (Jira Cloud style).

## Configuration

This command requires the **Global Jira Settings** (`JIRA_URL`). See the [main
README](../README.md#configuration) for details.

`JIRA_USERNAME` and `JIRA_API_TOKEN` are **not** used.

## Examples

```bash
maints open MAINT-41509
maints open DIG-30927
maints open MAINT-1 DIG-2
```

## Flags

`maints open` has no command-specific flags; use `maints open --help` for
usage.
