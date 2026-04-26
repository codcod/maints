# MAINTs CLI (`maints`)

A Jira maintenance toolkit: `maints triage` and `maints serve` use an AI agent to
evaluate issues; `maints dig` copies issues to another project with team and
links. Each command is documented in `docs/`.

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
git clone https://github.com/codcod/maints-triage.git
cd maints-triage
go build -o maints ./cmd/maints
```

With `just` (binary version follows the current git tag):

```bash
just build
```

Builds with no git context report `dev` as the version.

## Configuration

Set environment variables or use a **`.env`** file in the working directory.

1. Copy the template: `cp .env.example .env`
2. Fill in only what the commands you run need — requirements differ by command; see
   - [`docs/triage.md`](docs/triage.md) — Jira, Cursor agent, `MAINTS_HOME`, and files such as `checklist.md`
   - [`docs/serve.md`](docs/serve.md) — same agent/triage layer as `triage`, plus Jira polling
   - [`docs/dig.md`](docs/dig.md) — Jira, team field, and link settings

## General usage

- **[`maints triage`](docs/triage.md)** — Run AI triage on one or more Jira issues.
- **[`maints serve`](docs/serve.md)** — Poll Jira and triage new issues automatically.
- **[`maints dig`](docs/dig.md)** — Duplicate issues into another project with team and links.

```bash
maints --help
```

## License

MIT
