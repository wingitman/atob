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

## Neovim integration

See [atob.nvim](https://github.com/wingitman/atob.nvim).
