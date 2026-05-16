package config

import (
	"strings"
	"testing"
)

func TestKeybindEntriesMatchStruct(t *testing.T) {
	if err := KeybindEntriesMatchStruct(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultConfigIncludesUpdates(t *testing.T) {
	cfg := Default()
	if cfg.Keybinds.ShowUpdates != "U" {
		t.Fatalf("ShowUpdates = %q, want U", cfg.Keybinds.ShowUpdates)
	}

	toml := buildTOML(cfg)
	for _, want := range []string{"show_updates", "[updates]", "disable_checks", "current_commit", "repo_path", "terminal"} {
		if !strings.Contains(toml, want) {
			t.Fatalf("default TOML missing %q", want)
		}
	}
}
