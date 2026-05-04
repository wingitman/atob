//go:build ignore

package config

import "fmt"

func main() {
	cfg, err := Load()
	fmt.Printf("err=%v\nup=%q  quit=%q  live=%v  debounce=%d\npath=%s\n",
		err, cfg.Keybinds.Up, cfg.Keybinds.Quit, cfg.TUI.LivePreview,
		cfg.TUI.DebounceMs, ConfigPath())
	err2 := KeybindEntriesMatchStruct()
	fmt.Printf("KeybindEntriesMatchStruct: %v\n", err2)
}
