# MAINTs CLI (`maints`)

A Jira maintenance toolkit:
- `maints dash` lists your MAINT issues and related DIG work in the terminal.
- `maints open` opens Jira issue pages in the browser.
- `maints dig` copies issues to another project with team and links.
- `maints schedule` sets fix version(s) on DIG issues and comments on linked
  MAINTs.
- `maints release` follows up linked MAINTs after a fix version (comments; may
  close when all DIG work is done).

Extra:
- `maints triage` and `maints serve` use an AI agent to evaluate issues.

Command documentation is in the `docs/` folder where available; `maints
<command> --help` lists all flags.

## Installation

### Homebrew (macOS)

```bash
brew tap codcod/taps
brew install maints
```

Verify:
```bash
maints --version
maints --help
```

## Configuration

Set environment variables or use a **`.env`** file in the working directory.
Copy the template to get started: `cp .env.example .env`

### Global Jira Settings (used by all commands)
- `JIRA_URL`: Jira base URL (e.g., `https://your-company.atlassian.net`) —
  required for every command
- `JIRA_USERNAME` / `JIRA_API_TOKEN`: required for any command that calls the
  Jira API

### Agent Settings (used by `triage` and `serve`)
- `CURSOR_API_KEY`: API key for `cursor-agent`
- `MAINTS_HOME`: Optional directory for configuration files (defaults to
  `$XDG_CONFIG_HOME/maints` or `~/.config/maints/`)

### Dig Settings (used by `dig` and `dash`)
- `JIRA_TEAM_FIELD`: Custom field ID for Team (e.g., `customfield_14700`)
  (Required for `dig`)
- `JIRA_TEAM_ID`: Atlassian team UUID (Required for `dig`)
- `JIRA_DIG_ISSUE_TYPE`: Default target issue type (default: `Bug`)
- `JIRA_LINK_TYPE`: Default link type (default: `Solved by`)

## Commands

- **[`maints dash`](docs/dash.md)** — List your MAINT issues and linked
  DIG tickets in the terminal.
- **[`maints open`](docs/open.md)** — Open issue keys in the default browser
  (`MAINT-…`, `DIG-…`, etc.).
- **[`maints dig`](docs/dig.md)** — Create corresponding DIG ticket for a MAINT.
- **[`maints schedule`](docs/schedule.md)** — Set or remove fix version(s)
  on DIG issues; post a comment on the linked MAINT (see `maints schedule
  --help`).
- **[`maints release`](docs/release.md)** — Update linked MAINTs after release
  (see `maints release --help`).

## Commands (extra)

- **[`maints triage`](docs/triage.md)** — Run AI triage on one or more Jira
  issues.
- **[`maints serve`](docs/serve.md)** — Poll Jira and triage new issues
  automatically.

## License

MIT
