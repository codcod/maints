# Schedule command (`maints schedule`)

Set or remove fix version(s) on DIG issues; comment on linked MAINTs when Jira update succeeds.

## Configuration

This command requires the **Global Jira Settings** (`JIRA_URL`, `JIRA_USERNAME`, `JIRA_API_TOKEN`). See the [main README](../README.md#configuration) for details. It does **not** use `cursor-agent` or `CURSOR_API_KEY`.

`maints schedule` sets `fixVersions` on each DIG (adds names, or removes them with `--remove`) using the same issue link as `maints dig` / `maints dash` (default `Solved by`, or `$JIRA_LINK_TYPE`) to find the linked MAINT. After a successful update, it posts a "Patch Releases" comment on that MAINT for each version that was added or removed.

## Usage

Set fix versions on specific DIG issues:

```bash
maints schedule DIG-1 DIG-2 --version "DS 2025.09.5" --version "1.0"
```

Set fix versions on issues matching a JQL query:

```bash
maints schedule --query "project = DIG and status = 'In Progress'" --version "1.0"
```

Remove a fix version from a DIG issue instead of adding it:

```bash
maints schedule DIG-1 --version "1.0" --remove
```

## Flags

- `--version`: Fix version name to set or remove (required; use multiple times for several versions).
- `--query`: JQL to select DIG issues (mutually exclusive with issue keys on the command line).
- `--remove`: Remove the given fix version name(s) instead of adding them.
- `--link-type`: Link type to follow to MAINT (default: `$JIRA_LINK_TYPE` or `Solved by`).
- `--dig-project`: Project key to validate command-line and query results (default `DIG`).
- `--maint-project`: Key prefix of MAINT to comment on (default `MAINT`).
