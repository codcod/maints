package schedule

import "testing"

func TestVersionLooksLikePatch(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"DS 2025.09.2", true},
		{"DS 2025.09", false},
		{"2025.09.2", true},
		{"2025.09", false},
		{"DS 2025.09.10", true},
	}
	for _, tc := range cases {
		if got := versionLooksLikePatch(tc.in); got != tc.want {
			t.Errorf("versionLooksLikePatch(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestPatchVersionToMaintLTSFixVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"DS 2025.09.2", "2025.09-LTS", true},
		{"ds 2025.09.2", "2025.09-LTS", true},
		{"DS2025.09.2", "2025.09-LTS", true},
		{"2025.09.2", "2025.09-LTS", true},
		{"DS 2025.09.10", "2025.09-LTS", true},
		{"DS 2025.09", "2025.09-LTS", true},
		{"", "", false},
		{"DS", "", false},
	}
	for _, tc := range cases {
		got, ok := patchVersionToMaintLTSFixVersion(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%q: got %q, %v want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
