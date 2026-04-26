package dash

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/codcod/maints-triage/internal/dig"
	"github.com/codcod/maints-triage/internal/jira"
)

// DefaultJQL is the built-in filter for the MAINT (Flow) dashboard: current user's open MAINTs.
const DefaultJQL = `project = MAINT AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow) ` +
	`AND status not in (Done, Closed) AND assignee=currentUser() ORDER BY priority, created asc`

// DefaultJQLForAssignee is the same filter with assignee set to a specific Jira user (email, name, or account id string).
// assigneeS must be safe for a J-quoted JQL value (see jiraAssigneeJQLString).
func DefaultJQLForAssignee(assigneeS string) string {
	return `project = MAINT AND "Maint Component[Select List (cascading)]" IN cascadeOption(Flow) ` +
		`AND status not in (Done, Closed) AND assignee = ` + jiraAssigneeJQLString(assigneeS) + ` ORDER BY priority, created asc`
}

func jiraAssigneeJQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// maxSummaryRunes is the max display width of the SUMMARY column (MAINT and DIG lines).
const maxSummaryRunes = 50

// dashColGap is the gap between columns (spaces only so ANSI is applied per line, not per cell).
const dashColGap = "  "

// Row is one MAINT line plus linked DIG issues (fields follow table column order).
type Row struct {
	Key        string
	Priority   string
	Status     string
	Due        string
	Summary    string
	FixVersion string
	Assignee   string
	DIGs       []DigRow
}

// DigRow is a DIG issue shown under a MAINT.
type DigRow struct {
	Key        string
	Priority   string
	Status     string
	Summary    string
	FixVersion string
	Assignee   string
}

// Options control dash display and link resolution.
type Options struct {
	JQL        string
	DigProject string
	LinkType   string
	// Assignee, when set, selects the default JQL for that assignee (use with the built-in query, not with --jql).
	Assignee string
	// Debug prints issuelink metadata to errW (link types, keys) for troubleshooting.
	Debug bool
	// Columns is a comma-separated list of column names (e.g. key, priority, due). Empty means all default columns.
	Columns string
}

// Run fetches Jira and prints a text dashboard to w; link diagnostics to errW when o.Debug.
func Run(ctx context.Context, client *jira.Client, w, errW io.Writer, o Options) error {
	jql, err := effectiveDashJQL(o)
	if err != nil {
		return err
	}
	digProj := strings.ToUpper(strings.TrimSpace(o.DigProject))
	if digProj == "" {
		digProj = "DIG"
	}
	lt := strings.TrimSpace(o.LinkType)
	if lt == "" {
		lt = dig.DefaultLinkType()
	}
	hits, err := client.JQLSearchWithFields(ctx, jql, []string{
		"summary",
		"status",
		"duedate",
		"priority",
		"assignee",
		"issuelinks",
		"fixVersions",
	})
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		_, _ = fmt.Fprintln(w, "No issues matched the query.")
		return nil
	}
	// POST /search/jql does not always return issuelinks on issue objects. Reload per issue
	// (same as the Jira issue view) so "Solved by" DIG work appears on the dashboard.
	if err := enrichIssueLinksFromIssueAPI(ctx, client, hits); err != nil {
		return err
	}
	if o.Debug {
		writeDebugLinks(errW, hits)
	}
	rows, err := buildRows(hits, digProj, lt)
	if err != nil {
		return err
	}
	// One GET per DIG for authoritative status, assignee, and summary (issuelink embed is often partial).
	enriched, err := batchDigDetails(ctx, client, digKeysFromRows(rows))
	if err != nil {
		return err
	}
	for i := range rows {
		for j := range rows[i].DIGs {
			dk := rows[i].DIGs[j].Key
			if d, ok := enriched[dk]; ok {
				if d.status != "" {
					rows[i].DIGs[j].Status = d.status
				}
				rows[i].DIGs[j].Assignee = d.assignee
				if d.summary != "" {
					rows[i].DIGs[j].Summary = d.summary
				}
				if d.fixVersion != "" {
					rows[i].DIGs[j].FixVersion = d.fixVersion
				}
				if d.priority != "" {
					rows[i].DIGs[j].Priority = d.priority
				}
			}
		}
	}
	colSpecs, err := parseDashColumns(o.Columns)
	if err != nil {
		return err
	}
	printDashboard(w, rows, useColor(), colSpecs)
	return nil
}

// useColor is false when NO_COLOR is set (https://no-color.org/).
func useColor() bool {
	return os.Getenv("NO_COLOR") == ""
}

// effectiveDashJQL resolves --jql, --assignee, and the default query (mutually exclusive: --jql vs --assignee).
func effectiveDashJQL(o Options) (string, error) {
	jqlIn := strings.TrimSpace(o.JQL)
	assigneeIn := strings.TrimSpace(o.Assignee)
	if jqlIn != "" && assigneeIn != "" {
		return "", fmt.Errorf("use either --jql or --assignee, not both")
	}
	if jqlIn != "" {
		return jqlIn, nil
	}
	if assigneeIn != "" {
		return DefaultJQLForAssignee(assigneeIn), nil
	}
	return DefaultJQL, nil
}

func enrichIssueLinksFromIssueAPI(ctx context.Context, c *jira.Client, hits []jira.IssueJQL) error {
	if len(hits) == 0 {
		return nil
	}
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for i := range hits {
		i := i
		key := hits[i].Key
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fm, err := c.GetIssueFields(ctx, key, []string{"issuelinks"})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", key, err)
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			if v, ok := fm["issuelinks"]; ok {
				hits[i].Fields["issuelinks"] = v
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return firstErr
}

func writeDebugLinks(w io.Writer, hits []jira.IssueJQL) {
	for _, h := range hits {
		links, _ := h.Fields["issuelinks"].([]any)
		_, _ = fmt.Fprintf(w, "debug: %s: %d issuelink(s)\n", h.Key, len(links))
		for i, l := range links {
			m, _ := l.(map[string]any)
			if m == nil {
				_, _ = fmt.Fprintf(w, "  [%d] <not an object>\n", i)
				continue
			}
			lt, _ := m["type"].(map[string]any)
			var n, in, out string
			if lt != nil {
				n, in, out = asString(lt["name"]), asString(lt["inward"]), asString(lt["outward"])
			}
			ik := getIssueKeyFromLink(m, "inwardIssue")
			oke := getIssueKeyFromLink(m, "outwardIssue")
			_, _ = fmt.Fprintf(w, "  [%d] type name=%q inward=%q outward=%q inwardKey=%q outwardKey=%q\n", i, n, in, out, ik, oke)
		}
	}
}

// linkTypeMatch returns true if the Jira link type matches want (e.g. "Solved by").
// Jira stores a short name in type.name; the UI label is often in type.inward or type.outward
// (e.g. name "Solves" with inward "is solved by" / outward "solves"), so we match all three
// and allow substring match for long phrases.
func linkTypeMatch(want string, typeMap map[string]any) bool {
	if want == "" || typeMap == nil {
		return false
	}
	want = strings.TrimSpace(want)
	lowWant := strings.ToLower(want)
	for _, f := range []string{"name", "inward", "outward"} {
		s := asString(typeMap[f])
		if s == "" {
			continue
		}
		if strings.EqualFold(s, want) {
			return true
		}
		// "Solved by" should match "is solved by" on the inward description
		if strings.Contains(strings.ToLower(s), lowWant) {
			return true
		}
	}
	return false
}

type digDetails struct {
	status, assignee, summary, priority, fixVersion string
}

// batchDigDetails loads full status, assignee (displayName / email / name), and summary for DIG keys.
func batchDigDetails(ctx context.Context, c *jira.Client, keys []string) (map[string]digDetails, error) {
	out := make(map[string]digDetails, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	for _, k := range keys {
		k := k
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			st, as, su, pr, fv, err := c.GetIssueDashFields(ctx, k)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", k, err)
				}
				mu.Unlock()
				return
			}
			if strings.TrimSpace(as) == "" {
				as = "Unassigned"
			}
			mu.Lock()
			out[k] = digDetails{status: st, assignee: as, summary: su, priority: pr, fixVersion: fv}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func digKeysFromRows(rows []Row) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, r := range rows {
		for _, d := range r.DIGs {
			if _, ok := seen[d.Key]; ok {
				continue
			}
			seen[d.Key] = struct{}{}
			keys = append(keys, d.Key)
		}
	}
	return keys
}

func buildRows(hits []jira.IssueJQL, digProject, linkType string) ([]Row, error) {
	var rows []Row
	for _, h := range hits {
		status := fieldName(h.Fields, "status", "name")
		due := fieldDuedate(h.Fields)
		summary := asString(h.Fields["summary"])
		if summary == "" {
			summary = "—"
		}
		digs := digsSolvedBy(h.Key, h.Fields, linkType, digProject)
		prior := fieldName(h.Fields, "priority", "name")
		asm := jira.AssigneeString(h.Fields["assignee"])
		if strings.TrimSpace(asm) == "" {
			asm = "Unassigned"
		}
		fv := jira.FixVersionNamesString(h.Fields["fixVersions"])
		rows = append(rows, Row{
			Key: h.Key, Priority: prior, Status: status, Due: due, Summary: summary, FixVersion: fv, Assignee: asm, DIGs: digs,
		})
	}
	return rows, nil
}

func digsSolvedBy(maintKey string, fields map[string]any, wantLinkType, digProject string) (out []DigRow) {
	links, _ := fields["issuelinks"].([]any)
	if len(links) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	for _, l := range links {
		m, _ := l.(map[string]any)
		if m == nil {
			continue
		}
		lt, _ := m["type"].(map[string]any)
		if !linkTypeMatch(wantLinkType, lt) {
			continue
		}
		digKey, fieldSide, ok := pickDIGEnd(maintKey, m, digProject)
		if !ok || digKey == "" {
			continue
		}
		if _, dup := seen[digKey]; dup {
			continue
		}
		seen[digKey] = struct{}{}
		side, _ := m[fieldSide].(map[string]any)
		if side == nil {
			out = append(out, DigRow{Key: digKey})
			continue
		}
		inner, _ := side["fields"].(map[string]any)
		if inner == nil {
			out = append(out, DigRow{Key: digKey})
			continue
		}
		st := fieldName(inner, "status", "name")
		prio := fieldName(inner, "priority", "name")
		asg := jira.AssigneeString(inner["assignee"])
		if asg == "" {
			asg = "Unassigned"
		}
		sum := asString(inner["summary"])
		fv := jira.FixVersionNamesString(inner["fixVersions"])
		out = append(out, DigRow{Key: digKey, Priority: prio, Status: st, Summary: sum, FixVersion: fv, Assignee: asg})
	}
	return out
}

// pickDIGEnd returns the DIG key and the link map key (inwardIssue|outwardIssue) where the DIG side lives.
// Some Jira responses omit the current issue on one side; the viewed issue (maintKey) is then implicit.
func pickDIGEnd(maintKey string, link map[string]any, digProject string) (digKey, fieldSide string, ok bool) {
	inW, inOK := link["inwardIssue"].(map[string]any)
	outW, outOK := link["outwardIssue"].(map[string]any)
	// Jira can omit the "current" issue: only the remote end is in the JSON.
	if !inOK || inW == nil {
		if outOK && outW != nil {
			oke := getIssueKeyFromLink(link, "outwardIssue")
			if isDigKey(oke, digProject) && oke != "" {
				// Fetched issue is the inward; outward is DIG (common for Solved by).
				return oke, "outwardIssue", true
			}
		}
	}
	if !outOK || outW == nil {
		if inOK && inW != nil {
			ik := getIssueKeyFromLink(link, "inwardIssue")
			if isDigKey(ik, digProject) && ik != "" {
				return ik, "inwardIssue", true
			}
		}
	}
	ik := getIssueKeyFromLink(link, "inwardIssue")
	oke := getIssueKeyFromLink(link, "outwardIssue")
	if oke != "" && strings.EqualFold(ik, maintKey) && isDigKey(oke, digProject) {
		return oke, "outwardIssue", true
	}
	if ik != "" && strings.EqualFold(oke, maintKey) && isDigKey(ik, digProject) {
		return ik, "inwardIssue", true
	}
	return "", "", false
}

func getIssueKeyFromLink(link map[string]any, which string) string {
	if link == nil {
		return ""
	}
	m, _ := link[which].(map[string]any)
	if m == nil {
		return ""
	}
	if k, _ := m["key"].(string); k != "" {
		return strings.TrimSpace(k)
	}
	return ""
}

func isDigKey(key, digProject string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	p := strings.ToUpper(digProject) + "-"
	return strings.HasPrefix(upper, p)
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func fieldName(m map[string]any, a, b string) string {
	x, _ := m[a].(map[string]any)
	if x == nil {
		return ""
	}
	return asString(x[b])
}

func fieldDuedate(fields map[string]any) string {
	// duedate is string "YYYY-MM-DD" or absent
	if v, ok := fields["duedate"]; ok && v != nil {
		return asString(v)
	}
	return ""
}

// dashTableLine is one output row (header, MAINT, or DIG) with cells aligned to colSpecs.
type dashTableLine struct {
	cells  []string
	maint  bool
	header bool
	// dig is true for linked DIG sub-rows under a MAINT.
	dig        bool
	digSettled bool // Done or Closed: render STATUS cell white on green when color is on
}

func printDashboard(w io.Writer, rows []Row, color bool, colSpecs []columnSpec) {
	var lines []dashTableLine
	lines = append(lines, dashTableLine{
		cells:  dashTableHeaders(colSpecs),
		header: true,
	})
	for _, r := range rows {
		lines = append(lines, dashTableLine{
			cells: maintCells(r, colSpecs),
			maint: true,
		})
		for _, d := range r.DIGs {
			lines = append(lines, dashTableLine{
				cells:      digCells(d, colSpecs),
				dig:        true,
				digSettled: isDigSettledStatus(d.Status),
			})
		}
	}
	printPaddedTable(w, lines, color, colSpecs)
}

const (
	ansiRedFG        = "\x1b[31m"
	ansiReset        = "\x1b[0m"
	ansiBold         = "\x1b[1m"
	ansiWhiteOnRed   = "\x1b[97;41m"   // bright white on red background
	ansiWhiteOnGreen = "\x1b[97;42m"   // bright white on green background (Done/Closed DIG)
)

func printPaddedTable(w io.Writer, lines []dashTableLine, color bool, colSpecs []columnSpec) {
	widths := colWidths(lines)
	for _, ln := range lines {
		s := buildPaddedLine(ln.cells, widths, dashColGap)
		switch {
		case !color:
			_, _ = fmt.Fprintln(w, s)
		case ln.header:
			_, _ = fmt.Fprintln(w, ansiBold+s+ansiReset)
		case ln.maint:
			_, _ = fmt.Fprintln(w, formatMaintsRowColored(ln.cells, colSpecs, widths, dashColGap))
		case ln.dig && ln.digSettled:
			_, _ = fmt.Fprintln(w, formatDigSettledRow(ln.cells, colSpecs, widths, dashColGap))
		default:
			_, _ = fmt.Fprintln(w, s)
		}
	}
	_, _ = fmt.Fprintln(w)
}

// isDigSettledStatus is true for DIG Jira statuses shown as complete in the dash.
func isDigSettledStatus(s string) bool {
	s = strings.TrimSpace(s)
	return strings.EqualFold(s, "Done") || strings.EqualFold(s, "Closed")
}

// formatDigSettledRow is a DIG sub-row with the STATUS cell in white on green (Done/Closed only);
// other cells are uncolored. If the table omits the status column, no cell is highlighted.
func formatDigSettledRow(cells []string, colSpecs []columnSpec, widths []int, gap string) string {
	var b strings.Builder
	for i := 0; i < len(cells) && i < len(widths) && i < len(colSpecs); i++ {
		if i > 0 {
			b.WriteString(gap)
		}
		pad := padCellToRunes(cells[i], widths[i])
		if colSpecs[i].id == "status" {
			b.WriteString(ansiWhiteOnGreen)
			b.WriteString(pad)
			b.WriteString(ansiReset)
		} else {
			b.WriteString(pad)
		}
	}
	return b.String()
}

// formatMaintsRowColored is a red-foreground line for a MAINT row, with selected
// cells in white on red: STATUS for Open / AWAITING INPUT / TRIAGE, and DUE when
// the date is strictly before local today. Column position follows colSpecs.
// Only use when the outer table run has color (NO_COLOR is unset).
func formatMaintsRowColored(cells []string, colSpecs []columnSpec, widths []int, gap string) string {
	var b strings.Builder
	b.WriteString(ansiRedFG)
	for i := 0; i < len(cells) && i < len(widths) && i < len(colSpecs); i++ {
		if i > 0 {
			b.WriteString(gap)
		}
		pad := padCellToRunes(cells[i], widths[i])
		k := colSpecs[i].id
		highlight := (k == "status" && isMaintUrgentStatus(cells[i])) || (k == "due" && isPastDueCell(cells[i]))
		if highlight {
			b.WriteString(ansiReset)
			b.WriteString(ansiWhiteOnRed)
			b.WriteString(pad)
			b.WriteString(ansiReset)
			b.WriteString(ansiRedFG)
		} else {
			b.WriteString(pad)
		}
	}
	b.WriteString(ansiReset)
	return b.String()
}

// isMaintUrgentStatus is true for MAINT statuses that should read as "needs attention" in the dash.
func isMaintUrgentStatus(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	switch {
	case strings.EqualFold(s, "Open"):
		return true
	case strings.EqualFold(s, "AWAITING INPUT"):
		return true
	case strings.EqualFold(s, "TRIAGE"):
		return true
	default:
		return false
	}
}

// isPastDueCell is true for a YYYY-MM-dd (or datetime prefix) in the DUE cell strictly before start of local today.
func isPastDueCell(s string) bool {
	ds := jiraYMDString(s)
	if ds == "" {
		return false
	}
	d, err := time.ParseInLocation("2006-01-02", ds, time.Local)
	if err != nil {
		return false
	}
	now := time.Now().In(time.Local)
	day0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return d.Before(day0)
}

// jiraYMDString returns the YYYY-MM-dd part of a duedate field, or "" if not a date.
func jiraYMDString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return ""
	}
	if i := strings.IndexByte(s, 'T'); i > 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func padCellToRunes(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func dashCellOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func colWidths(lines []dashTableLine) []int {
	if len(lines) == 0 {
		return nil
	}
	n := len(lines[0].cells)
	w := make([]int, n)
	for _, ln := range lines {
		for i, cell := range ln.cells {
			if i >= n {
				break
			}
			if u := utf8.RuneCountInString(cell); u > w[i] {
				w[i] = u
			}
		}
	}
	return w
}

func buildPaddedLine(cells []string, widths []int, gap string) string {
	var b strings.Builder
	for i := 0; i < len(cells) && i < len(widths); i++ {
		if i > 0 {
			b.WriteString(gap)
		}
		c := cells[i]
		if utf8.RuneCountInString(c) < widths[i] {
			c = c + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c))
		}
		b.WriteString(c)
	}
	return b.String()
}

func truncRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) > n-1 {
		return string(r[:n-1]) + "…"
	}
	return s
}
