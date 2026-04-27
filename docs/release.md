# Release command (`maints release`)

Run after a patch has been published. It selects DIG issues in a **fix
version**, checks that each is **Done** or **Closed**, then updates any linked
**MAINT** that `maints dig` would connect with the same **Solved by**–style
link (default `JIRA_LINK_TYPE` or `Solved by`).

## Default scope

The command uses this JQL (version and project are parameters / flags):

```jql
project = DIG AND fixVersion = "<your version>"
```

Override the DIG project with `--dig-project` if needed.

## Behavior

1. For each DIG in that scope, if the status is not **Done** or **Closed**, a
   **warning** is printed and that DIG is **not** used to drive MAINT updates.
2. For each **Done** or **Closed** DIG, the tool finds the linked **MAINT**
   (same link matching rules as `maints dig` and `maints dash`).
3. For each **unique** MAINT (deduplicated across DIGs in this run), it loads
   all links of that type to DIGs. If there are no such links, a warning is
   printed.
4. If the MAINT is **already** **Done** or **Closed**, the command logs that
   and **skips** further action (no duplicate comments).
5. If **every** linked DIG in that set is **Done** or **Closed**:
   - The MAINT is **transitioned** to status **Done** (with resolution **Done**
     when the workflow allows; the client may retry with no resolution or with
     **Fixed** if Jira requires it).
   - A comment is added: *All patches have been released now, closing.*
   - **By default**, the transition and closing comment run **only** when the
     MAINT’s **assignee** is the **authenticated Jira user** (the same person
     as in JQL `currentUser()`). If the assignee is someone else, the command
     **skips** the close and logs a message. Use **`--supervisor`** to allow
     closing **any** assignee.
6. If **any** of those DIGs is still not **Done** or **Closed**:
   - A comment is added: *Patch [fix version] has been released, keeping MAINT
     open until all patches are released.* (This “keep open” comment is not
     restricted by assignee.)

## Configuration

Requires the **Global Jira Settings** from the [main
README](../README.md#configuration) (`JIRA_URL`, `JIRA_USERNAME`,
`JIRA_API_TOKEN`). Optional: `JIRA_LINK_TYPE` (same as `maints dig` / `maints
dash`).

## Examples

```bash
maints release 'DS 2025.09.2'
maints release "DS 2025.09.2" --dig-project DIG --maint-project MAINT
# Close MAINTs for any assignee (not only issues assigned to you)
maints release 'DS 2025.09.2' --supervisor
```

## Flags

- `--link-type`: Link name to follow to MAINT (default: `$JIRA_LINK_TYPE` or
  `Solved by`).
- `--dig-project`: DIG project key (default `DIG`).
- `--maint-project`: MAINT project key prefix (default `MAINT`).
- `--supervisor`: When set, **closing** a MAINT in step 5 is allowed regardless
  of assignee. Without it, only MAINTs assigned to you are closed.

Use `maints release --help` for the full flag list.
