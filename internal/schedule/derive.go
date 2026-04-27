package schedule

import (
	"strings"
)

// patchVersionToMaintLTSFixVersion maps a DIG patch fix version (e.g. "DS 2025.09.2") to the
// MAINT Jira fixVersions name: strip a leading "DS" prefix (optional space after it), drop a
// trailing numeric patch segment (… .<n>), then append "-LTS" (e.g. "2025.09-LTS").
func patchVersionToMaintLTSFixVersion(patch string) (string, bool) {
	s := strings.TrimSpace(patch)
	if s == "" {
		return "", false
	}
	if len(s) >= 2 && strings.EqualFold(s[:2], "DS") {
		s = strings.TrimSpace(s[2:])
	}
	if s == "" {
		return "", false
	}
	parts := strings.Split(s, ".")
	if len(parts) >= 3 {
		last := parts[len(parts)-1]
		if last != "" && isAllDigits(last) {
			parts = parts[:len(parts)-1]
			s = strings.Join(parts, ".")
		}
	}
	if s == "" {
		return "", false
	}
	return s + "-LTS", true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// versionLooksLikePatch is true when the version string has a trailing numeric patch segment
// after stripping a leading "DS" prefix (e.g. "DS 2025.09.2" → true, "DS 2025.09" → false).
func versionLooksLikePatch(version string) bool {
	s := strings.TrimSpace(version)
	if s == "" {
		return false
	}
	if len(s) >= 2 && strings.EqualFold(s[:2], "DS") {
		s = strings.TrimSpace(s[2:])
	}
	if s == "" {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return false
	}
	last := parts[len(parts)-1]
	return last != "" && isAllDigits(last)
}
