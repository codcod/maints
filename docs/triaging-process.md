# The Triage Process

This document describes in detail how the automated maintenance ticket triage
process works in the `maints` CLI (`maints triage` and `maints serve`).

## 1. Overview

The `maints` tool automates the initial evaluation of Jira maintenance tickets.
When a new defect is reported by an internal team or a customer, an AI agent
evaluates the ticket's quality, completeness, and validity to determine the
appropriate next step:

- **Accept**: The ticket is a valid defect and contains enough context to begin
  engineering analysis.
- **Reject**: The ticket is not a valid defect (e.g., it is a feature request,
  a question, targets an unsupported version, or describes expected product
  behavior).
- **Request Info**: The ticket appears to be a valid defect but is missing
  critical information required to reproduce or investigate it.

This automated process ensures consistent standards, reduces manual triage
overhead, and speeds up the routing of valid issues to development squads.

## 2. Core Components

The triage process is orchestrated by a Go application (`internal/triage`) and
executed by an AI agent (usually Claude) using the Cursor agent framework. The
behavior is defined by a set of configuration files typically stored in
`$XDG_CONFIG_HOME/maints` (or `$MAINTS_HOME`):

- **`triage-prompt.md`**: The system instructions defining the agent's persona,
  the step-by-step evaluation workflow, and output format requirements.
- **`checklist.md`**: A list of criteria (e.g., Issue Type, Supported Version,
  Steps to Reproduce, Expected Behavior) that a ticket must satisfy. Items are
  categorized as `[REQUIRED]`, `[CONDITIONAL]`, or `[OPTIONAL]`.
- **`kb-index.md`**: A curated index of the external product Knowledge Base. It
  maps components to documentation and outlines common "working as designed"
  patterns to prevent false defect reports.
- **`fields-mapping.json`**: (Optional) Maps custom Jira fields to standard
  metadata.

## 3. The Triage Workflow

When `maints triage` or `maints serve` runs for a given set of Jira issue keys,
it executes the following steps:

### Step A: Data Fetching and Setup

1. **Jira API Fetch**: The Go application connects to the Jira REST API to
   download the issue's summary, description, metadata (components, versions,
   priority), and all comments.
2. **Workspace Creation**: For each issue, an isolated directory is created
   (e.g., `triaged-maints/MAINT-12345/`).
3. **File Provisioning**:
   - The Jira issue data is formatted as markdown and saved as
     `issue-MAINT-12345.md`.
   - The `checklist.md` and `kb-index.md` files are copied into the workspace.
   - The AI agent is launched with its root workspace set to this directory.

### Step B: AI Agent Evaluation

The AI agent processes the ticket following the strict instructions in
`triage-prompt.md`.

#### 1. Knowledge Base Consultation (Step 0)
The agent reads `kb-index.md` to establish context. It checks the reported
version against the supported release lifecycle. It looks up the affected
component to read the relevant AsciiDoc product documentation (symlinked into
the workspace) to verify if the reported behavior is actually the designed
intent of the product.

#### 2. Early Rejection Screening (Step 1)
The agent performs a "gate check" before evaluating the full checklist:
- If the ticket is a feature request, question, or describes
  working-as-designed behavior confirmed by the KB, it is immediately rejected.
- If the ticket targets an end-of-life (EOL) product version, it is immediately
  rejected.

#### 3. Item-by-Item Analysis (Step 2)
If the ticket passes the gates, the agent evaluates it against every item in
`checklist.md`. For each item, the agent:
- Searches the issue description and comments for evidence.
- Evaluates `[CONDITIONAL]` triggers (e.g., "Are logs required? Only if it's a
  backend issue, not a mobile UI bug").
- Assigns a rating: `complete`, `partial`, `missing`, or `na` (not applicable).
- Extracts verbatim quotes from the Jira issue to justify the rating.

#### 4. Verdict and Decision Generation (Step 3)
Based on the individual ratings, the agent calculates an overall outcome:
- **Verdict**: `PASS_CHECKLIST_COMPLIANCE` (all required/applicable items are
  present) or `FAIL_CHECKLIST_COMPLIANCE`.
- **Decision**: `accept`, `reject`, or `request_info`. Note that a ticket can
  fail strict checklist compliance but still be accepted if the missing items
  are minor and don't block engineering investigation.
- **Confidence Score**: A rating from 0.0 to 1.0 indicating the agent's
  certainty. Low confidence flags the ticket for manual human review
  (`review_required: true`).

### Step C: Parsing and Validation

1. **JSON Extraction**: The agent emits a structured JSON object containing its
   item ratings, overall decision, evidence, and any questions for the
   reporter.
2. **Go Validation**: The `internal/triage/evaluation.go` module parses this
   JSON. It runs safety checks (e.g., warning if the agent passed the checklist
   but still decided to reject or request info).
3. **Fallback**: If the agent fails to produce valid JSON, the system degrades
   gracefully and returns the raw AI text response.

### Step D: Output Generation

For each triaged ticket, the tool produces two artifacts in the workspace:
- **`report-{KEY}.md`**: A human-readable markdown report detailing the agent's
  analysis, item ratings, extracted quotes, and final decision.
- **`report-{KEY}.json`**: A machine-readable JSON representation of the
  outcome.

Finally, the master process aggregates all individual results into a single
`TriageOutcomeBatch` JSON document and prints it to `stdout` for downstream
consumption by CI/CD pipelines or reporting tools.

## 4. Nuances and Best Practices

- **Reducing "Request Info" Noise**: The checklist relies heavily on
  `[CONDITIONAL]` requirements. A mobile UI bug does not need server logs, and
  an internal test report does not require a customer hosting environment. The
  prompt instructs the agent to be generous with `na` (Not Applicable) to
  prevent unnecessarily blocking valid tickets.
- **Knowledge Base Integration**: A major source of wasted engineering time is
  investigating "bugs" that turn out to be expected product behavior. By
  injecting a curated subset of the documentation (`kb-index.md`) directly into
  the agent's context, the system can autonomously reject tickets that describe
  intentional architectural designs, third-party limitations, or known
  configuration issues.
