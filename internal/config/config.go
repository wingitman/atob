// Package config loads and writes the atob configuration file.
// The config file is created automatically on first launch at a
// platform-appropriate location under the "delbysoft" vendor directory.
//
// Paths:
//
//	Linux:   ~/.config/delbysoft/atob.toml
//	macOS:   ~/Library/Application Support/delbysoft/atob.toml
//	Windows: %APPDATA%\delbysoft\atob.toml
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration struct.
type Config struct {
	Keybinds Keybinds `toml:"keybinds"`
	TUI      TUI      `toml:"tui"`
	Output   Output   `toml:"output"`
	Updates  Updates  `toml:"updates"`
	Themes   Themes   `toml:"themes"`
}

// Keybinds holds every user-configurable key binding.
// Values use Bubble Tea key names: "up", "down", "tab", "shift+tab",
// "ctrl+c", "pgup", "pgdown", single characters like "q" or "/".
type Keybinds struct {
	Up          string `toml:"up"`
	Down        string `toml:"down"`
	PageUp      string `toml:"page_up"`
	PageDown    string `toml:"page_down"`
	HalfUp      string `toml:"half_up"`
	HalfDown    string `toml:"half_down"`
	Top         string `toml:"top"`
	Bottom      string `toml:"bottom"`
	NextPane    string `toml:"next_pane"`
	PrevPane    string `toml:"prev_pane"`
	Search      string `toml:"search"`
	ClearSearch string `toml:"clear_search"`
	Select      string `toml:"select"`
	Run         string `toml:"run"`
	CopyOutput  string `toml:"copy_output"`
	SaveOutput  string `toml:"save_output"`
	OpenConfig  string `toml:"open_config"`
	ShowUpdates string `toml:"show_updates"`
	ClearInput  string `toml:"clear_input"`
	Quit        string `toml:"quit"`
	QuitAlt     string `toml:"quit_alt"`
	Theme       string `toml:"theme"`
}

// TUI holds TUI behaviour settings.
type TUI struct {
	LivePreview bool `toml:"live_preview"`
	DebounceMs  int  `toml:"debounce_ms"`
}

// Output holds settings for saving output files.
type Output struct {
	SaveDir string `toml:"save_dir"`
}

// Updates holds update-check and installer preferences.
type Updates struct {
	DisableChecks bool   `toml:"disable_checks"`
	CurrentCommit string `toml:"current_commit"`
	RepoPath      string `toml:"repo_path"`
	Terminal      string `toml:"terminal"`
}

// keybindEntry pairs a TOML key name with its inline comment.
// The order here controls both write order and migration detection.
var keybindEntries = []struct{ key, comment string }{
	{"up", "move cursor up in list / scroll output up"},
	{"down", "move cursor down in list / scroll output down"},
	{"page_up", "page up in list or output"},
	{"page_down", "page down in list or output"},
	{"half_up", "scroll output up half-page"},
	{"half_down", "scroll output down half-page"},
	{"top", "jump to top of list"},
	{"bottom", "jump to bottom of list"},
	{"next_pane", "focus next pane (list → input → output)"},
	{"prev_pane", "focus previous pane"},
	{"search", "enter search mode in list"},
	{"clear_search", "exit search mode / clear filter"},
	{"select", "select converter and focus input pane"},
	{"run", "manually trigger conversion (or force re-run)"},
	{"copy_output", "copy output to clipboard"},
	{"save_output", "save output to file"},
	{"open_config", "open atob.toml in $EDITOR"},
	{"show_updates", "show update history and installers"},
	{"clear_input", "clear the input pane"},
	{"quit", "quit atob"},
	{"quit_alt", "quit (not active when input pane is focused)"},
	{"theme", "open theme picker"},
}

var tuiEntries = []string{"live_preview", "debounce_ms"}
var outputEntries = []string{"save_dir"}
var updateEntries = []string{"disable_checks", "current_commit", "repo_path", "terminal"}
var themeEntries = []string{"theme_name", "theme_file"}

// Default returns the default configuration.
func Default() Config {
	return Config{
		Keybinds: Keybinds{
			Up:          "up",
			Down:        "down",
			PageUp:      "pgup",
			PageDown:    "pgdown",
			HalfUp:      "ctrl+u",
			HalfDown:    "ctrl+d",
			Top:         "g",
			Bottom:      "G",
			NextPane:    "tab",
			PrevPane:    "shift+tab",
			Search:      "/",
			ClearSearch: "esc",
			Select:      "enter",
			Run:         "ctrl+r",
			CopyOutput:  "y",
			SaveOutput:  "s",
			OpenConfig:  "o",
			ShowUpdates: "U",
			ClearInput:  "ctrl+l",
			Quit:        "ctrl+c",
			QuitAlt:     "q",
			Theme:       "T",
		},
		TUI: TUI{
			LivePreview: true,
			DebounceMs:  150,
		},
		Output: Output{
			SaveDir: "",
		},
		Updates: Updates{
			DisableChecks: false,
			CurrentCommit: "",
			RepoPath:      "",
			Terminal:      "",
		},
		Themes: Themes{
			ThemeName: "terminal",
			ThemeFile: filepath.Join(ConfigDir(), "themes.toml"),
		},
	}
}

// ConfigDir returns the platform-appropriate config directory.
func ConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return ""
		}
		return filepath.Join(home, ".config", "delbysoft")
	}
	return filepath.Join(base, "delbysoft")
}

// ConfigPath returns the full path to the atob config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "atob.toml")
}

// Load reads the config file, creating it with defaults if it doesn't exist.
// Errors are non-fatal — the function always returns a usable Config.
func Load() (Config, error) {
	path := ConfigPath()
	cfg := Default()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// First launch: create the directory and write the annotated default file.
		if mkErr := os.MkdirAll(ConfigDir(), 0o755); mkErr != nil {
			return cfg, nil // silently use in-memory defaults
		}
		if wErr := WriteDefault(path); wErr != nil {
			return cfg, nil
		}
		_ = EnsureThemesFile(cfg)
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Default(), fmt.Errorf("parsing config %s: %w", path, err)
	}

	applyKeybindDefaults(&cfg.Keybinds)
	applyTUIDefaults(&cfg.TUI)
	if cfg.Themes.ThemeName == "" {
		cfg.Themes.ThemeName = Default().Themes.ThemeName
	}
	if cfg.Themes.ThemeFile == "" {
		cfg.Themes.ThemeFile = Default().Themes.ThemeFile
	}

	if needsMigration(path) {
		_ = writeMigrated(path, cfg)
	}
	_ = EnsureThemesFile(cfg)

	return cfg, nil
}

// WriteDefault writes the fully-annotated default config to path.
func WriteDefault(path string) error {
	return os.WriteFile(path, []byte(buildTOML(Default())), 0o644)
}

// applyKeybindDefaults fills any empty keybind field with its default value.
func applyKeybindDefaults(k *Keybinds) {
	d := Default().Keybinds
	v := reflect.ValueOf(k).Elem()
	dv := reflect.ValueOf(d)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.String() == "" {
			f.SetString(dv.Field(i).String())
		}
	}
}

// applyTUIDefaults fills zero-value TUI fields with defaults.
func applyTUIDefaults(t *TUI) {
	d := Default().TUI
	if t.DebounceMs <= 0 {
		t.DebounceMs = d.DebounceMs
	}
}

// keybindValues returns a map of TOML key → current value for all keybinds.
func keybindValues(k Keybinds) map[string]string {
	return map[string]string{
		"up":           k.Up,
		"down":         k.Down,
		"page_up":      k.PageUp,
		"page_down":    k.PageDown,
		"half_up":      k.HalfUp,
		"half_down":    k.HalfDown,
		"top":          k.Top,
		"bottom":       k.Bottom,
		"next_pane":    k.NextPane,
		"prev_pane":    k.PrevPane,
		"search":       k.Search,
		"clear_search": k.ClearSearch,
		"select":       k.Select,
		"run":          k.Run,
		"copy_output":  k.CopyOutput,
		"save_output":  k.SaveOutput,
		"open_config":  k.OpenConfig,
		"show_updates": k.ShowUpdates,
		"clear_input":  k.ClearInput,
		"quit":         k.Quit,
		"quit_alt":     k.QuitAlt,
		"theme":        k.Theme,
	}
}

// needsMigration returns true if the config file is missing any known key.
func needsMigration(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	for _, e := range keybindEntries {
		if !fileContainsKey(content, e.key) {
			return true
		}
	}
	for _, key := range tuiEntries {
		if !fileContainsKey(content, key) {
			return true
		}
	}
	for _, key := range outputEntries {
		if !fileContainsKey(content, key) {
			return true
		}
	}
	for _, key := range updateEntries {
		if !fileContainsKey(content, key) {
			return true
		}
	}
	for _, key := range themeEntries {
		if !fileContainsKeyInSection(content, "themes", key) {
			return true
		}
	}
	return false
}

func fileContainsKey(content, key string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" ") {
			return true
		}
	}
	return false
}

func fileContainsKeyInSection(content, section, key string) bool {
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inSection = trimmed == "["+section+"]"
			continue
		}
		if inSection && !strings.HasPrefix(trimmed, "#") &&
			(strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" ")) {
			return true
		}
	}
	return false
}

// writeMigrated rewrites the config file, preserving user values but adding
// any missing keys with their defaults.
func writeMigrated(path string, cfg Config) error {
	return os.WriteFile(path, []byte(buildTOML(cfg)), 0o644)
}

// RecordUpdateMetadata stores the installed commit and source repo path without
// changing user-facing preferences.
func RecordUpdateMetadata(commit, repoPath string) error {
	cfg, err := Load()
	if err != nil {
		cfg = Default()
	}
	if commit != "" {
		cfg.Updates.CurrentCommit = commit
	}
	if repoPath != "" {
		cfg.Updates.RepoPath = repoPath
	}
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		return err
	}
	return writeMigrated(ConfigPath(), cfg)
}

// buildTOML generates the full annotated TOML string for a given Config.
func buildTOML(cfg Config) string {
	vals := keybindValues(cfg.Keybinds)

	// Find the longest key name for column alignment.
	maxLen := 0
	for _, e := range keybindEntries {
		if len(e.key) > maxLen {
			maxLen = len(e.key)
		}
	}

	var sb strings.Builder
	sb.WriteString("# atob configuration\n")
	sb.WriteString("# Key values: use names like \"up\", \"down\", \"left\", \"right\", \"enter\",\n")
	sb.WriteString("# \"tab\", \"shift+tab\", \"ctrl+c\", \"pgup\", \"pgdown\", or single characters.\n")
	sb.WriteString("# Vim-style example: up=\"k\"  down=\"j\"  half_up=\"ctrl+u\"  half_down=\"ctrl+d\"\n")
	sb.WriteString("\n[keybinds]\n")

	for _, e := range keybindEntries {
		val := vals[e.key]
		line := fmt.Sprintf("%-*s = %-14s # %s\n", maxLen, e.key, fmt.Sprintf("%q", val), e.comment)
		sb.WriteString(line)
	}

	sb.WriteString("\n[tui]\n")
	sb.WriteString(fmt.Sprintf("live_preview = %v  # update output as you type (false = manual ctrl+r only)\n", cfg.TUI.LivePreview))
	sb.WriteString(fmt.Sprintf("debounce_ms  = %d   # milliseconds to wait after keypress before converting\n", cfg.TUI.DebounceMs))

	sb.WriteString("\n[output]\n")
	sb.WriteString(fmt.Sprintf("save_dir = %q  # directory for saved output files (empty = ~/Downloads)\n", cfg.Output.SaveDir))

	sb.WriteString("\n[updates]\n")
	sb.WriteString(fmt.Sprintf("disable_checks = %v  # true disables startup update checks\n", cfg.Updates.DisableChecks))
	sb.WriteString(fmt.Sprintf("current_commit = %q  # installed app commit, maintained by atob\n", cfg.Updates.CurrentCommit))
	sb.WriteString(fmt.Sprintf("repo_path = %q  # source checkout used for updates\n", cfg.Updates.RepoPath))
	sb.WriteString(fmt.Sprintf("terminal = %q  # optional terminal command for detached updates\n", cfg.Updates.Terminal))

	sb.WriteString("\n[themes]\n")
	sb.WriteString(fmt.Sprintf("theme_name = %q  # terminal, or a named theme from theme_file\n", cfg.Themes.ThemeName))
	sb.WriteString(fmt.Sprintf("theme_file = %q  # shared Delbysoft theme file\n", cfg.Themes.ThemeFile))
	sb.WriteString("# Optional overrides applied after the selected theme.\n")
	sb.WriteString("# primary = \"#7C9EF0\"\n")
	sb.WriteString("# accent = \"#F0A47C\"\n")
	sb.WriteString("# muted = \"#666688\"\n")
	sb.WriteString("# error = \"#F07C7C\"\n")
	sb.WriteString("# success = \"#7CF09C\"\n")
	sb.WriteString("# border = \"#444466\"\n")
	sb.WriteString("# selected_foreground = \"#EEEEFF\"\n")
	sb.WriteString("# brand_primary = \"#FFFFFF\"\n")
	sb.WriteString("# brand_secondary = \"#5865F2\"\n")
	sb.WriteString("# selector = \"#FFFFFF\"\n")

	return sb.String()
}

// SaveDirResolved returns the resolved save directory path,
// defaulting to ~/Downloads if SaveDir is empty.
func (o Output) SaveDirResolved() string {
	if o.SaveDir != "" {
		return o.SaveDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	dir := filepath.Join(home, "Downloads")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// Verify that keybindEntries covers every field in the Keybinds struct.
// This is called from a test but also acts as compile-time documentation.
func KeybindEntriesMatchStruct() error {
	entryKeys := make(map[string]bool, len(keybindEntries))
	for _, e := range keybindEntries {
		entryKeys[e.key] = true
	}
	t := reflect.TypeOf(Keybinds{})
	var missing []string
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("toml")
		if !entryKeys[tag] {
			missing = append(missing, tag)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("keybindEntries missing fields: %s", strings.Join(missing, ", "))
	}
	// Check reverse: entries without a matching struct field
	fieldTags := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		fieldTags[t.Field(i).Tag.Get("toml")] = true
	}
	var extra []string
	for _, e := range keybindEntries {
		if !fieldTags[e.key] {
			extra = append(extra, e.key)
		}
	}
	if len(extra) > 0 {
		return fmt.Errorf("keybindEntries has extra keys not in struct: %s", strings.Join(extra, ", "))
	}
	return nil
}
