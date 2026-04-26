package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// TriageHome returns the triage configuration directory, resolving in priority order:
//  1. $TRIAGE_HOME if set.
//  2. $XDG_CONFIG_HOME/triage (falling back to ~/.config/triage when XDG_CONFIG_HOME is unset).
func TriageHome() (string, error) {
	if th := os.Getenv("TRIAGE_HOME"); th != "" {
		return th, nil
	}
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		xdgConfigHome = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfigHome, "triage"), nil
}
