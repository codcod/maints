# MAINTs CLI (`maints`)

A Jira maintenance toolkit:
- `maints triage` and `maints serve` use an AI agent to evaluate issues.
- `maints dig` copies issues to another project with team and links.
- `maints dash` lists your MAINT issues and related DIG work in the terminal.
- `maints open` opens Jira issue pages in the browser.
- `maints fixversion` sets fix version(s) on DIG issues and comments on linked MAINTs.

Command documentation is in the `docs/` folder where available; `maints <command> --help` lists all flags.

## Installation

### Homebrew (macOS)

```bash
brew tap codcod/taps
brew install maints
```

### From source

#### Prerequisites
- Go 1.25+

#### Install

```bash
git clone https://github.com/codcod/maints-triage.git
cd maints-triage
go install ./cmd/maints
```

Verify:
```bash
maints --version
maints --help
```

### Build from source

To build a binary in the current directory (without `go install` to `$GOPATH/bin`):

```bash
go build -o maints ./cmd/maints
```

With `just` (binary version follows the current git tag):
```bash
just build
```

Builds with no git context report `dev` as the version.

## Configuration

Set environment variables or use a **`.env`** file in the working directory. Copy the template to get started: `cp .env.example .env`

### Global Jira Settings (used by all commands)
- `JIRA_URL`: Jira base URL (e.g., `https://your-company.atlassian.net`) — required for every command
- `JIRA_USERNAME` / `JIRA_API_TOKEN`: required for any command that calls the Jira API (not for [`maints open`](docs/open.md), which only opens browse URLs)

### Agent Settings (used by `triage` and `serve`)
- `CURSOR_API_KEY`: API key for `cursor-agent`
- `MAINTS_HOME`: Optional directory for configuration files (defaults to `$XDG_CONFIG_HOME/maints` or `~/.config/maints/`)

### Dig Settings (used by `dig` and `dash`)
- `JIRA_TEAM_FIELD`: Custom field ID for Team (e.g., `customfield_14700`) (Required for `dig`)
- `JIRA_TEAM_ID`: Atlassian team UUID (Required for `dig`)
- `JIRA_DIG_ISSUE_TYPE`: Default target issue type (default: `Bug`)
- `JIRA_LINK_TYPE`: Default link type (default: `Solved by`)

## Commands

- **[`maints triage`](docs/triage.md)** — Run AI triage on one or more Jira issues.
- **[`maints serve`](docs/serve.md)** — Poll Jira and triage new issues automatically.
- **[`maints dig`](docs/dig.md)** — Duplicate issues into another project with team and links.
- **[`maints dash`](docs/dash.md)** — List your MAINT Flow issues and linked DIG tickets in the terminal.
- **[`maints open`](docs/open.md)** — Open issue keys in the default browser (`MAINT-…`, `DIG-…`, etc.).
- **[`maints fixversion`](docs/fixversion.md)** — Set or remove fix version(s) on DIG issues; post a comment on the linked MAINT (see `maints fixversion --help`).

```bash
maints --help
```

## License

MIT
