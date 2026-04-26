package dash

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// defaultDashColumnOrder is used when --columns is omitted.
var defaultDashColumnOrder = []string{
	"key", "priority", "status", "due", "summary", "fixversion", "assignee",
}

var dashColumnHeader = map[string]string{
	"key":        "KEY",
	"priority":   "PRIORITY",
	"status":     "STATUS",
	"due":        "DUE",
	"summary":    "SUMMARY",
	"fixversion": "FIXVERSION",
	"assignee":   "ASSIGNEE",
}

// reColumnWidth matches e.g. SUMMARY[20] or "summary[ 20 ]" (name must be a known column, width only for summary).
var reColumnWidth = regexp.MustCompile(`(?i)^\s*([a-z_][a-z0-9_]*)\s*\[\s*(\d+)\s*\]\s*$`)

// knownDashColumns for error messages
func knownDashColumns() string {
	var k []string
	for id := range dashColumnHeader {
		k = append(k, id)
	}
	sort.Strings(k)
	return strings.Join(k, ", ")
}

// columnSpec is one output column. summaryMax is only used when id is "summary":
// 0 means the default (maxSummaryRunes); a positive value sets the max rune width
// (from syntax SUMMARY[n] in --columns).
type columnSpec struct {
	id         string
	summaryMax int
}

// parseDashColumns parses a comma-separated --columns value. Empty s selects
// the default full set (summary uses default 50 runes). Order is preserved.
func parseDashColumns(s string) ([]columnSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		out := make([]columnSpec, len(defaultDashColumnOrder))
		for i, id := range defaultDashColumnOrder {
			out[i] = columnSpec{id: id}
		}
		return out, nil
	}
	seen := make(map[string]struct{})
	var out []columnSpec
	for _, part := range strings.Split(s, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		spec, err := parseOneColumn(t)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[spec.id]; dup {
			return nil, fmt.Errorf("duplicate column %q", spec.id)
		}
		seen[spec.id] = struct{}{}
		out = append(out, spec)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one column is required in --columns")
	}
	return out, nil
}

func parseOneColumn(t string) (columnSpec, error) {
	if m := reColumnWidth.FindStringSubmatch(t); len(m) == 3 {
		name, err := normalizeBareColumnName(m[1])
		if err != nil {
			return columnSpec{}, err
		}
		n, err := strconv.Atoi(m[2])
		if err != nil || n < 1 {
			return columnSpec{}, fmt.Errorf("invalid width in %q: use a positive integer, e.g. summary[20]", t)
		}
		if name != "summary" {
			return columnSpec{}, fmt.Errorf("column %q: only \"summary\" supports a [N] width (e.g. summary[20])", t)
		}
		return columnSpec{id: "summary", summaryMax: n}, nil
	}
	id, err := normalizeColumnToken(t)
	if err != nil {
		return columnSpec{}, err
	}
	return columnSpec{id: id}, nil
}

// normalize bare name from a bracketed token (no brackets).
func normalizeBareColumnName(s string) (string, error) {
	low := strings.ToLower(strings.TrimSpace(s))
	low = strings.ReplaceAll(low, " ", "_")
	low = strings.ReplaceAll(low, "-", "_")
	if low == "fix_version" {
		low = "fixversion"
	}
	if _, ok := dashColumnHeader[low]; !ok {
		return "", fmt.Errorf("unknown column name %q in [N] form: valid names are: %s", s, knownDashColumns())
	}
	return low, nil
}

func normalizeColumnToken(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	low := strings.ToLower(s)
	low = strings.ReplaceAll(low, " ", "_")
	low = strings.ReplaceAll(low, "-", "_") // e.g. fix-version → fix_version
	if low == "fix_version" {
		low = "fixversion"
	}
	if _, ok := dashColumnHeader[low]; !ok {
		return "", fmt.Errorf("unknown column %q: valid names are: %s", s, knownDashColumns())
	}
	return low, nil
}

func (c columnSpec) effectiveSummaryMax() int {
	if c.id != "summary" {
		return maxSummaryRunes
	}
	if c.summaryMax > 0 {
		return c.summaryMax
	}
	return maxSummaryRunes
}

func maintCells(r Row, cols []columnSpec) []string {
	out := make([]string, len(cols))
	for i := range cols {
		out[i] = maintCell(r, cols[i])
	}
	return out
}

func digCells(d DigRow, cols []columnSpec) []string {
	out := make([]string, len(cols))
	for i := range cols {
		out[i] = digCell(d, cols[i])
	}
	return out
}

func maintCell(r Row, c columnSpec) string {
	switch c.id {
	case "key":
		return r.Key
	case "priority":
		if strings.TrimSpace(r.Priority) == "" {
			return "—"
		}
		return r.Priority
	case "status":
		if strings.TrimSpace(r.Status) == "" {
			return "—"
		}
		return r.Status
	case "due":
		if strings.TrimSpace(r.Due) == "" {
			return "—"
		}
		return r.Due
	case "summary":
		return truncRunes(r.Summary, c.effectiveSummaryMax())
	case "fixversion":
		return dashCellOrDash(r.FixVersion)
	case "assignee":
		if strings.TrimSpace(r.Assignee) == "" {
			return "Unassigned"
		}
		return r.Assignee
	default:
		return "—"
	}
}

const digKeyIndent = "  "

func digCell(d DigRow, c columnSpec) string {
	switch c.id {
	case "key":
		return digKeyIndent + d.Key
	case "priority":
		if strings.TrimSpace(d.Priority) == "" {
			return "—"
		}
		return d.Priority
	case "status":
		if strings.TrimSpace(d.Status) == "" {
			return "—"
		}
		return d.Status
	case "due":
		return "—"
	case "summary":
		maxW := c.effectiveSummaryMax()
		sumD := truncRunes(strings.TrimSpace(d.Summary), maxW)
		if strings.TrimSpace(sumD) == "" {
			return "—"
		}
		return sumD
	case "fixversion":
		return dashCellOrDash(d.FixVersion)
	case "assignee":
		if strings.TrimSpace(d.Assignee) == "" {
			return "Unassigned"
		}
		return d.Assignee
	default:
		return "—"
	}
}

func dashTableHeaders(cols []columnSpec) []string {
	h := make([]string, len(cols))
	for i, c := range cols {
		if c.id == "summary" && c.summaryMax > 0 {
			h[i] = fmt.Sprintf("SUMMARY[%d]", c.summaryMax)
			continue
		}
		if title, ok := dashColumnHeader[c.id]; ok {
			h[i] = title
		} else {
			h[i] = strings.ToUpper(c.id)
		}
	}
	return h
}
