package open

import (
	"testing"
)

func TestIssueBrowseURL(t *testing.T) {
	if got, want := IssueBrowseURL("https://acme.atlassian.net/", "MAINT-1"), "https://acme.atlassian.net/browse/MAINT-1"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidateKey(t *testing.T) {
	for _, k := range []string{"MAINT-41509", "DIG-30927", "A-1"} {
		if _, err := ValidateKey(k); err != nil {
			t.Errorf("%q: %v", k, err)
		}
	}
	if _, err := ValidateKey(""); err == nil {
		t.Fatal("empty key")
	}
	if _, err := ValidateKey("MAINT"); err == nil {
		t.Fatal("no id")
	}
}
