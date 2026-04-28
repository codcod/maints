package dash

import "strings"

// splitCommaList splits on commas, trims each part, and drops empty tokens.
func splitCommaList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// filterMaintRows keeps MAINT rows whose status and priority match the allowlists.
// If statuses is non-empty, Status must equal one entry (case-insensitive).
// If priorities is non-empty, Priority must equal one entry (case-insensitive).
// DIG sub-rows stay attached to kept MAINT rows.
func filterMaintRows(rows []Row, statuses, priorities []string) []Row {
	if len(statuses) == 0 && len(priorities) == 0 {
		return rows
	}
	var out []Row
	for _, r := range rows {
		if len(statuses) > 0 && !matchesAnyFold(r.Status, statuses) {
			continue
		}
		if len(priorities) > 0 && !matchesAnyFold(r.Priority, priorities) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func matchesAnyFold(field string, allowed []string) bool {
	field = strings.TrimSpace(field)
	for _, a := range allowed {
		if strings.EqualFold(field, strings.TrimSpace(a)) {
			return true
		}
	}
	return false
}
