# triage

A CLI tool that uses `cursor-agent` to triage Jira maintenance issues against a checklist.

It fetches issue data from Jira, prepares a temporary workspace with the issue content and a checklist, and then instructs the AI agent to verify completeness.

## Installation

### Prerequisites

- Go 1.25+
- `cursor-agent` CLI installed and available in your `$PATH`
- A Jira account (Cloud or Data Center)
- A Cursor API key (for the agent)

### Install

```bash
git clone https://github.com/yourusername/triager.git
cd triager
go install .
```

Verify installation:

```bash
triage --help
```

### Build from Source

If you prefer to build the binary without installing it to your `$GOPATH/bin`:

```bash
git clone https://github.com/yourusername/triager.git
cd triager
go build -o triage .
```

This creates a `triage` binary in the current directory. You can then run it with `./triage`.

You can also use `just`:

```bash
just build
```

## Configuration

The tool requires credentials for both Jira and Cursor. You can provide these via environment variables or a `.env` file in the current directory.

1. Copy the example file:
   ```bash
   cp .env.example .env
   ```
2. Edit `.env` and fill in your details:
   ```bash
   # Jira Configuration
   JIRA_URL=https://your-company.atlassian.net
   JIRA_USERNAME=your-email@company.com
   JIRA_API_TOKEN=your-jira-api-token

   # Cursor Agent API Key
   CURSOR_API_KEY=your-cursor-api-key
   ```

## Usage

### Basic Triage

Triage a single issue:

```bash
triage MAINT-123
```

Triage multiple issues in one go:

```bash
triage MAINT-123 MAINT-456 MAINT-789
```

### Custom Checklist

By default, `triage` looks for a checklist in the following order:

1. Path passed via `--checklist`
2. `$XDG_CONFIG_HOME/triage/checklist.md` (defaults to `~/.config/triage/checklist.md`)
3. `./checklist.md` in the current directory

To use an explicit checklist:

```bash
triage --checklist ./my-custom-checklist.md MAINT-123
```

### AI Model Selection

Specify which AI model the agent should use (e.g., `sonnet-4`, `gpt-4o`):

```bash
triage --model sonnet-4 MAINT-123
```

### Output Format

Output the triage report as JSON (useful for piping to other tools):

```bash
triage --output json MAINT-123 | jq .
```

## How It Works

1. **Fetch**: Connects to the Jira REST API to retrieve the issue summary, description, comments, and metadata (status, priority, versions, components, etc.).
2. **Prepare**: Creates a `triaged-maints/KEY/` directory in the current working directory and writes two files into it:
   - `issue-KEY.md`: A Markdown-formatted representation of the Jira issue.
   - `checklist.md`: The triage checklist.
3. **Analyze**: Invokes `cursor-agent` in headless mode with `triaged-maints/KEY/` as its workspace. The agent reads both files and evaluates the issue against each checklist item.
4. **Report**: The agent's response is printed to stdout and also saved as `triaged-maints/KEY/report-KEY.md`.

After a run the directory is kept so you can review or commit the artefacts:

```
triaged-maints/
└── MAINT-123/
    ├── issue-MAINT-123.md
    ├── checklist.md
    └── report-MAINT-123.md
```

## License

MIT
