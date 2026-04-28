package dash

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// maintNeedsAttention matches the MAINT row highlights in formatMaintsRowColored:
// Critical/Blocker priority, past due, or Open / AWAITING INPUT / TRIAGE status.
func maintNeedsAttention(r Row) bool {
	return isMaintCriticalPriority(r.Priority) || isPastDueCell(r.Due) || isMaintUrgentStatus(r.Status)
}

type assigneeAgg struct {
	total int
	needs int
}

func printSupervisorSummary(w io.Writer, rows []Row, color bool) {
	if len(rows) == 0 {
		return
	}
	printSummaryHeader(w, "=== SUPERVISOR SUMMARY ===", color)

	total := len(rows)
	var nCritical, nPastDue, nUrgentStatus, nNeeds int
	statusCounts := make(map[string]int)
	byAssignee := make(map[string]*assigneeAgg)

	for _, r := range rows {
		st := strings.TrimSpace(r.Status)
		if st == "" {
			st = "—"
		}
		statusCounts[st]++

		a := r.Assignee
		if strings.TrimSpace(a) == "" {
			a = "Unassigned"
		}
		agg, ok := byAssignee[a]
		if !ok {
			agg = &assigneeAgg{}
			byAssignee[a] = agg
		}
		agg.total++

		crit := isMaintCriticalPriority(r.Priority)
		past := isPastDueCell(r.Due)
		urg := isMaintUrgentStatus(r.Status)
		if crit {
			nCritical++
		}
		if past {
			nPastDue++
		}
		if urg {
			nUrgentStatus++
		}
		if crit || past || urg {
			nNeeds++
			agg.needs++
		}
	}

	printSummaryHeader(w, "OVERVIEW", color)
	printKV(w, "Total issues", fmt.Sprintf("%d", total), false, color)
	needsVal := fmt.Sprintf("%d (Critical/Blocker %d, Past due %d, Urgent status %d)", nNeeds, nCritical, nPastDue, nUrgentStatus)
	highlightNeeds := color && nNeeds > 0
	printKV(w, "Needs action", needsVal, highlightNeeds, color)
	_, _ = fmt.Fprintln(w)

	printSummaryHeader(w, "BY STATUS (count, descending)", color)
	printStatusLine(w, statusCounts)
	_, _ = fmt.Fprintln(w)

	printSummaryHeader(w, "BY ASSIGNEE (needs action, then total, then name)", color)
	nameW := maxAssigneeNameWidth(byAssignee)
	var list []struct {
		name  string
		total int
		needs int
	}
	for k, a := range byAssignee {
		list = append(list, struct {
			name  string
			total int
			needs int
		}{name: k, total: a.total, needs: a.needs})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].needs != list[j].needs {
			return list[i].needs > list[j].needs
		}
		if list[i].total != list[j].total {
			return list[i].total > list[j].total
		}
		return strings.ToLower(list[i].name) < strings.ToLower(list[j].name)
	})
	for _, row := range list {
		nameCol := padCellToRunes(row.name, nameW)
		_, _ = fmt.Fprintf(w, "%s : %d total  (%d needs action)\n", nameCol, row.total, row.needs)
	}
	_, _ = fmt.Fprintln(w)
}

func printSummaryHeader(w io.Writer, title string, color bool) {
	if color {
		_, _ = fmt.Fprintf(w, "%s%s%s\n", ansiBold, title, ansiReset)
	} else {
		_, _ = fmt.Fprintln(w, title)
	}
}

func printKV(w io.Writer, label, value string, highlightValue bool, color bool) {
	label = strings.TrimSpace(label)
	if color && highlightValue {
		_, _ = fmt.Fprintf(w, "%s : %s%s%s\n", label, ansiWhiteOnRed, value, ansiReset)
		return
	}
	_, _ = fmt.Fprintf(w, "%s : %s\n", label, value)
}

func printStatusLine(w io.Writer, counts map[string]int) {
	type pair struct {
		name  string
		count int
	}
	var pairs []pair
	for k, v := range counts {
		pairs = append(pairs, pair{name: k, count: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return strings.ToLower(pairs[i].name) < strings.ToLower(pairs[j].name)
	})
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteString("  |  ")
		}
		fmt.Fprintf(&b, "%s: %d", p.name, p.count)
	}
	_, _ = fmt.Fprintln(w, b.String())
}

func maxAssigneeNameWidth(m map[string]*assigneeAgg) int {
	maxW := utf8.RuneCountInString("ASSIGNEE")
	for k := range m {
		if u := utf8.RuneCountInString(k); u > maxW {
			maxW = u
		}
	}
	return maxW
}
