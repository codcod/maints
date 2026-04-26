# Dash command (`maints dash`)

Prints a terminal dashboard of your MAINT Flow issues and linked DIG work.

## Configuration

This command requires the **Global Jira Settings** and optionally uses the `JIRA_LINK_TYPE` setting. See the [main README](../README.md#configuration) for details. It does not use `cursor-agent` or `CURSOR_API_KEY`.

Link names for related DIG work default the same way as `maints dig`: `JIRA_LINK_TYPE`, or `JIRA_SOLVES_LINK_TYPE`, or **Solved by**. You can override this per run with `--link-type`.

## Default JQL

If you do not pass `--jql` or `--user`, `maints dash` uses the following query:

```jql
project = MAINT AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow)
AND status not in (Done, Closed)
AND assignee=currentUser()
ORDER BY priority, created asc
```

## Output

The table has six columns: **KEY** (indented for linked DIGs), **PRIORITY**, **STATUS**, **DUE** (`—` for DIGs), **SUMMARY** (truncated to 50 runes for MAINT and DIG lines), and **ASSIGNEE** (last column). 

Styling details:
- The header line is **bold**.
- Each **MAINT** data row is **red**; DIG rows are normal.
- On a MAINT row, **STATUS** is **white on red** if it is Open, AWAITING INPUT, or TRIAGE (case-insensitive).
- A **DUE** value strictly before today (local) is **white on red** in that cell. 
- Set **`NO_COLOR`** in the environment to disable all styling.

JQL search does not always return `issuelinks`, so the command also performs a `GET` per MAINT to load them. Every **DIG** is re-fetched to get the authoritative priority, status, assignee, and summary (assignee uses `displayName`, then `name` / `email`).

Link matching checks `type.name`, `inward`, and `outward` descriptions, including substring matches. If DIG rows are missing, run `maints dash --debug` and check stderr.

## Flags

- `--jql`: Replace the default JQL entirely (mutually exclusive with `--user`).
- `--user`: Use the built-in MAINT-Flow JQL, but filter `assignee` to this value (Jira user email, display string, or id). Mutually exclusive with `--jql`.
- `--dig-project`: Project key for "DIG" work items (default `DIG`).
- `--link-type`: Link type name in Jira (default from env or `Solved by`).
- `--debug`: Print `issuelinks` type names and keys to stderr (no secret data).

## Examples

```bash
maints dash
maints dash --user colleague@example.com
maints dash --dig-project DIG --link-type "Solved by"
maints dash --jql 'project = MAINT AND assignee = currentUser() ORDER BY created ASC'
```
