# atob

A universal CLI conversion tool written in Go. Pipe text through any of the
built-in converters or add your own in minutes.

## Installation

### Linux / macOS — one-liner

```sh
curl -fsSL https://raw.githubusercontent.com/wingitman/atob/main/install.sh | bash
```

Or download and run manually:

```sh
./install.sh                         # installs to ~/.local/bin
./install.sh /usr/local/bin          # custom directory (may need sudo)
INSTALL_DIR=/usr/local/bin ./install.sh
```

The script builds from source if Go is on your PATH, otherwise downloads a
pre-built binary from GitHub Releases.

### Windows — PowerShell

```powershell
irm https://raw.githubusercontent.com/wingitman/atob/main/install.ps1 | iex
```

Or download and run:

```powershell
.\install.ps1                        # installs to %USERPROFILE%\.local\bin
.\install.ps1 -InstallDir C:\tools
```

No admin rights required — the installer adds the directory to the user PATH
via the registry.

### Build from source with make

Requires Go 1.21+.

```sh
git clone https://github.com/wingitman/atob
cd atob
make install                         # builds + installs to ~/.local/bin
make install INSTALL_DIR=/usr/local/bin
```

| Target | Description |
|---|---|
| `make` / `make build` | Build binary for current platform |
| `make install` | Build + install to `$INSTALL_DIR` (default `~/.local/bin`) |
| `make uninstall` | Remove installed binary |
| `make clean` | Remove build artefacts |
| `make release` | Cross-compile all platforms into `dist/` |

### Uninstall

Linux / macOS:
```sh
./uninstall.sh                       # removes from ~/.local/bin
make uninstall                       # same via make
```

Windows:
```powershell
Remove-Item "$env:USERPROFILE\.local\bin\atob.exe"
```

## TUI

Run `atob` with no arguments to open the interactive TUI:

```sh
atob
```

Pre-load a file into the input pane:

```sh
atob ./mydata.json          # reads file, opens TUI with content pre-loaded
atob /usr/bin/ls            # binary file — pre-selects inspect/hexdump/strings
```

### Layout

Three-pane interface that fills your terminal:

```
┌─────────────────┬──────────────────────┬────────────────────────────────────┐
│  CONVERTERS     │  INPUT               │  OUTPUT  ·  json → yaml            │
│                 │                      │                                    │
│  json → yaml  ▶ │  {"name":"atob"}     │  name: atob                        │
│  json → toml    │                      │                                    │
│  yaml → json    │  auto-detect → yaml  │                                    │
│  / search_      │                      │                                    │
├─────────────────┴──────────────────────┴────────────────────────────────────┤
│  tab:pane  /:search  enter:select  ctrl+r:run  y:copy  s:save  q:quit        │
└─────────────────────────────────────────────────────────────────────────────┘
```

- **Left** — searchable converter list. `/` to search, arrows to navigate, `Enter` to select
- **Middle** — input area. Type text or a file path; live preview updates as you type
- **Right** — scrollable output with scroll indicator. `y` to copy, `s` to save to file

### Keybinds (default)

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle between panes |
| `↑` / `↓` | Navigate list / scroll output line-by-line |
| `Ctrl+U` / `Ctrl+D` | Scroll output half-page |
| `g` / `G` | Jump to top / bottom of list |
| `/` | Enter search mode in the converter list |
| `Esc` | Exit search mode |
| `Enter` | Select converter, focus input pane |
| `Ctrl+R` | Manually trigger conversion (or force re-run) |
| `Ctrl+V` | Paste from clipboard into input pane |
| `y` | Copy output to clipboard |
| `s` | Save output to file (shows format picker) |
| `q` / `Ctrl+C` | Quit |

All keybinds are configurable — see [Configuration](#configuration) below.

## Usage

```sh
# auto-detect input type, convert to target
atob 'hello world' base64
atob '{"a":1}' yaml
atob '{"a":1}' toml
atob 'hello_world' camel
atob 'hello' sha256
atob '255' hex           # decimal 255 → ff
atob '1741000000' epoch text

# explicit from → to (when you want to be unambiguous)
atob '{"a":1}' json yaml
atob 'hello_world' snake camel
atob 'aGVsbG8=' base64 text

# stdin variants
echo '{"a":1}' | atob yaml
echo '{"a":1}' | atob json toml

# file-based conversions
atob csv xlsx input.csv output.xlsx
atob xlsx csv input.xlsx output.csv

# discover everything
atob list
atob list --json    # machine-readable (used by atob.nvim)
```

### Type names

Pass any of these as `<from>` or `<to>`:

| Type | Aliases |
|---|---|
| `json` | `js` |
| `yaml` | `yml` |
| `toml` | |
| `xml` | |
| `csv` | |
| `xlsx` | `excel`, `xls` |
| `base64` | `b64` |
| `hex` | |
| `url` | |
| `html` | |
| `binary` | `bin` |
| `octal` | `oct` |
| `decimal` | `dec`, `num`, `int` |
| `text` | `plain`, `str`, `raw` |
| `md5` | |
| `sha1` | |
| `sha256` | |
| `sha512` | |
| `gzip` | `gz` |
| `zlib` | |
| `uuid` | `guid` |
| `epoch` | `timestamp`, `ts`, `unix` |
| `camel` | `camelcase` |
| `pascal` | `pascalcase` |
| `snake` | `snakecase` |
| `kebab` | `kebabcase` |
| `screaming-snake` | `upper-snake` |
| `screaming-kebab` | `upper-kebab` |

## Converters

| Category | Converters |
|---|---|
| **encoding** | `base64-encode`, `base64-decode`, `hex-encode`, `hex-decode`, `url-encode`, `url-decode`, `html-encode`, `html-decode` |
| **case** | `case-camel`, `case-pascal`, `case-snake`, `case-screaming-snake`, `case-kebab`, `case-screaming-kebab`, `case-title` |
| **hash** | `hash-md5`, `hash-sha1`, `hash-sha256`, `hash-sha512` |
| **numbers** | `dec-bin`, `bin-dec`, `dec-oct`, `oct-dec`, `dec-hex`, `hex-dec` |
| **formats** | `json-yaml`, `yaml-json`, `json-toml`, `toml-json`, `json-xml`, `xml-json`, `json-csv`, `csv-json`, `json-pretty`, `json-minify`, `csv-xlsx`*, `xlsx-csv`* |
| **identity** | `uuid-generate`, `uuid-validate`, `epoch-human`, `human-epoch`, `epoch-now` |
| **compression** | `gzip-compress`, `gzip-decompress`, `zlib-compress`, `zlib-decompress` |

\* file-path based

## Adding your own converter

See [`conversions/README.md`](conversions/README.md) for the full guide.
The short version:

1. Create a `.go` file in the appropriate `conversions/<category>/` directory
2. Implement the `Converter` interface (4 methods + an `init()` registration)
3. `go build` — your converter is live

## Configuration

The config file is created automatically on first launch.

| OS | Path |
|---|---|
| Linux | `~/.config/delbysoft/atob.toml` |
| macOS | `~/Library/Application Support/delbysoft/atob.toml` |
| Windows | `%APPDATA%\delbysoft\atob.toml` |

### Default config

```toml
# atob configuration
# Key values: use names like "up", "down", "left", "right", "enter",
# "tab", "shift+tab", "ctrl+c", "pgup", "pgdown", or single characters.
# Vim-style example: up="k"  down="j"  half_up="ctrl+u"  half_down="ctrl+d"

[keybinds]
up           = "up"           # move cursor up in list / scroll output up
down         = "down"         # move cursor down in list / scroll output down
page_up      = "pgup"         # page up in list or output
page_down    = "pgdown"       # page down in list or output
half_up      = "ctrl+u"       # scroll output up half-page
half_down    = "ctrl+d"       # scroll output down half-page
top          = "g"            # jump to top of list
bottom       = "G"            # jump to bottom of list
next_pane    = "tab"          # focus next pane (list → input → output)
prev_pane    = "shift+tab"    # focus previous pane
search       = "/"            # enter search mode in list
clear_search = "esc"          # exit search mode / clear filter
select       = "enter"        # select converter and focus input pane
run          = "ctrl+r"       # manually trigger conversion (or force re-run)
copy_output  = "y"            # copy output to clipboard
save_output  = "s"            # save output to file
quit         = "ctrl+c"       # quit atob
quit_alt     = "q"            # quit (not active when input pane is focused)

[tui]
live_preview = true  # update output as you type (false = manual ctrl+r only)
debounce_ms  = 150   # milliseconds to wait after keypress before converting

[output]
save_dir = ""  # directory for saved output files (empty = ~/Downloads)
```

### Vim-style keybinds

Add this to your config:

```toml
[keybinds]
up        = "k"
down      = "j"
half_up   = "ctrl+u"
half_down = "ctrl+d"
top       = "g"
bottom    = "G"
```

### Saved output files

Pressing `s` in the output pane opens a format picker and saves the output to
`~/Downloads/atob-<converter>-<timestamp>.<ext>` — e.g.
`~/Downloads/atob-yaml-20260504-153012.yaml`.

Set a custom save directory:

```toml
[output]
save_dir = "/home/you/conversions"
```

## Neovim integration

See [atob.nvim](https://github.com/wingitman/atob.nvim).
