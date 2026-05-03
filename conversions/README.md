# Adding Your Own Converters

This directory contains all conversion logic for `atob`. Each category has its
own subdirectory, and each file implements one or more related converters.

The system uses a simple interface-based registry — drop a new `.go` file in
the right category folder and it will be automatically picked up when you
rebuild.

---

## The `Converter` interface (text-based)

Use this for anything that reads a string and returns a string — encoding,
hashing, case transformation, number bases, etc.

```go
type Converter interface {
    Name()        string
    Category()    string
    Description() string
    Convert(input string) (string, error)
}
```

### Step-by-step: adding a text converter

1. **Create a file** in the appropriate category directory.  
   If no category fits, create a new subdirectory (e.g. `conversions/mycat/`).

2. **Declare the package** matching the directory name:
   ```go
   package mycat
   ```

3. **Define a struct** for each converter (zero-value structs are fine):
   ```go
   type rot13 struct{}
   ```

4. **Implement the interface**:
   ```go
   func (rot13) Name()        string { return "rot13" }
   func (rot13) Category()    string { return "encoding" }
   func (rot13) Description() string { return "Apply ROT13 cipher to text" }

   func (rot13) Convert(input string) (string, error) {
       // your logic here
       return result, nil
   }
   ```

5. **Register in `init()`**:
   ```go
   func init() {
       conversions.Register(rot13{})
   }
   ```

6. **Import the package** in `cmd/root.go` (one blank import line):
   ```go
   _ "github.com/wingitman/atob/conversions/mycat"
   ```

7. **Rebuild**:
   ```sh
   go build -o atob .
   ./atob list          # your converter appears here
   echo 'hello' | ./atob rot13
   ```

---

## The `FileConverter` interface (file-based)

Use this when inputs or outputs are binary files that cannot cleanly travel
through stdin/stdout (e.g. xlsx, zip, PDF).

```go
type FileConverter interface {
    Name()        string
    Category()    string
    Description() string
    ConvertFile(inputPath, outputPath string) error
}
```

Usage on the CLI:
```sh
atob my-converter input.bin output.txt
```

### Step-by-step: adding a file converter

Follow the same steps as above, but:

- Implement `ConvertFile(inputPath, outputPath string) error` instead of `Convert`.
- Register with `conversions.RegisterFile(myConverter{})` instead of `conversions.Register`.

---

## Naming conventions

| Convention | Example |
|---|---|
| Use lowercase kebab-case for `Name()` | `"json-yaml"`, `"base64-encode"` |
| Category matches the directory name | `"encoding"`, `"formats"` |
| Bidirectional converters are two separate structs | `jsonToYAML{}` + `yamlToJSON{}` |
| Strip trailing newlines from input in `Convert()` | `strings.TrimRight(input, "\n")` |

---

## Complete minimal example

```go
// conversions/encoding/rot13.go
package encoding

import (
    "strings"
    "github.com/wingitman/atob/conversions"
)

func init() { conversions.Register(rot13{}) }

type rot13 struct{}

func (rot13) Name()        string { return "rot13" }
func (rot13) Category()    string { return "encoding" }
func (rot13) Description() string { return "Apply ROT13 substitution cipher" }

func (rot13) Convert(input string) (string, error) {
    return strings.Map(func(r rune) rune {
        switch {
        case r >= 'a' && r <= 'z':
            return 'a' + (r-'a'+13)%26
        case r >= 'A' && r <= 'Z':
            return 'A' + (r-'A'+13)%26
        }
        return r
    }, strings.TrimRight(input, "\n")), nil
}
```

Because this file lives in the `encoding` package it needs **no changes** to
`cmd/root.go` — the existing blank import already covers it.
