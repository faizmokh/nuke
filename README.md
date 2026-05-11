# nuke

A Go CLI for cleaning Xcode DerivedData and Swift Package Manager caches on macOS.

## Install

**Homebrew:**

```bash
brew install faizmokh/tap/nuke
```

**From source:**

```bash
go install github.com/faizmokh/nuke@latest
```

## Usage

```bash
nuke derived    # Clean ~/Library/Developer/Xcode/DerivedData
nuke spm        # Clean ~/Library/Caches/org.swift.swiftpm
nuke all        # Clean both
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--yes` | `-y` | Skip confirmation prompt |
| `--dry-run` | | Show what would be deleted without deleting |
| `--version` | | Print version |

### Examples

```bash
nuke derived -y       # Nuke DerivedData without prompt
nuke all --dry-run    # Preview what would be deleted
```

## Build

```bash
go build -o nuke .
```

## Test

```bash
go test ./... -v
```
