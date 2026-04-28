# Dash command (`maints dash`)

Prints a terminal dashboard of your MAINT Flow issues and linked DIG work.

## Configuration

This command requires the **Global Jira Settings** and optionally uses the
`JIRA_LINK_TYPE` setting. See the [main README](../README.md#configuration) for
details. It does not use `cursor-agent` or `CURSOR_API_KEY`.

Link names for related DIG work default the same way as `maints dig`:
`JIRA_LINK_TYPE`, or `JIRA_SOLVES_LINK_TYPE`, or **Solved by**. You can
override this per run with `--link-type`.

## Default JQL

If you do not pass `--jql`, `--assignee`, or `--supervisor`, `maints dash` uses the
following query:

```jql
project = MAINT
AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow)
AND status not in (Done, Closed)
AND assignee=currentUser()
ORDER BY priority, created asc
```

With **`--supervisor`**, the same Flow filter is used but **without** an assignee
restriction, so you can see open Flow MAINTs for **all** assignees (ordered by
assignee, then priority, then created):

```jql
project = MAINT
AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow)
AND status not in (Done, Closed)
ORDER BY assignee, priority, created asc
```

After the table, **`--supervisor`** also prints a **supervisor summary**:
total MAINT count; **needs action** (rows that would be highlighted in the
table: Blocker/Critical, past-due, or status Open / AWAITING INPUT / TRIAGE),
with a breakdown by category; counts **by Jira status** (descending); and per-
**assignee** totals with needs-action counts (sorted by needs action, then
total, then name). `NO_COLOR` disables styling in this block too.

## Output

By default the table includes these columns, in order: **KEY** (indented for
linked DIGs), **PRIORITY**, **STATUS**, **DUE** (`—` for DIGs), **SUMMARY**
(truncated to 50 runes for MAINT and DIG lines), **SCHEDULED** (Jira
`fixVersions` names, comma‑separated when there are several), and **ASSIGNEE**.
Use `--columns` to list only the columns you want, in the order you list them
(see [Flags](#flags)). For **SUMMARY**, the default max width is 50 runes; add
a bracketed number to set a different max (e.g. `SUMMARY[20]` in the header
when you pass `summary[20]`). Empty fix versions show as `—`. 

Styling details:
- The header line is **bold**.
- Each **MAINT** data row is **red**; DIG rows are normal.
- On a MAINT row, **STATUS** is **white on red** if it is Open, AWAITING INPUT,
  or TRIAGE (case-insensitive).
- On a MAINT row, **PRIORITY** is **white on red** if it is Blocker or Critical
  (case-insensitive).
- A **DUE** value strictly before today (local) is **white on red** in that cell. 
- Set **`NO_COLOR`** in the environment to disable all styling.

JQL search does not always return `issuelinks`, so the command also performs a
`GET` per MAINT to load them. Every **DIG** is re-fetched to get the
authoritative priority, status, assignee, and summary (assignee uses
`displayName`, then `name` / `email`).

Link matching checks `type.name`, `inward`, and `outward` descriptions,
including substring matches. If DIG rows are missing, run `maints dash --debug`
and check stderr.

## Flags

- `--jql`: Replace the default JQL entirely (mutually exclusive with
  `--assignee` and `--supervisor`).
- `--assignee`: Use the built-in MAINT-Flow JQL, but filter `assignee` to this
  value (Jira user email, display string, or id). Mutually exclusive with
  `--jql` and `--supervisor`.
- `--supervisor`: Use the built-in MAINT-Flow JQL **without** limiting
  `assignee` (team overview). After the dashboard table, prints aggregate
  statistics. Mutually exclusive with `--jql` and `--assignee`.
- `--no-dig`: Print only **MAINT** rows (no linked DIG sub-rows, no DIG detail
  fetches, no per-MAINT issue-link reload). JQL and column flags behave as usual.
- `--dig-project`: Project key for "DIG" work items (default `DIG`).
- `--link-type`: Link type name in Jira (default from env or `Solved by`).
- `--columns`: Comma‑separated column names (case‑insensitive; optional
  spaces). Allowed names: `key`, `priority`, `status`, `due`, `summary`,
  `scheduled` (also `scheduled_version` or `scheduled-version`); `assignee`.
  The **summary** column defaults to 50 runes; use `summary[N]` to cap at `N`
  runes (e.g. `summary[20]`). Only `summary` may use a `[N]` suffix. Example:
  `--columns "key, summary[20], priority"`.
- `--debug`: Print `issuelinks` type names and keys to stderr (no secret data).

## Examples

```bash
maints dash
maints dash --no-dig
maints dash --assignee 'Name Surname'
maints dash --supervisor
maints dash --columns "key, priority, due"
maints dash --columns "key, summary[20], scheduled, assignee"
maints dash --dig-project DIG --link-type "Solved by"
maints dash --jql 'project = MAINT AND assignee = currentUser() ORDER BY created ASC'
```
