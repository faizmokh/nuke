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
nuke derived    # Interactively clean ~/Library/Developer/Xcode/DerivedData
nuke spm        # Clean ~/Library/Caches/org.swift.swiftpm
nuke all        # Clean both
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--yes` | `-y` | Skip confirmation prompt |
| `--dry-run` | | Show what would be deleted without deleting |
| `--version` | | Print version |

### DerivedData Flags

| Flag | Description |
|------|-------------|
| `--all` | Delete all DerivedData entries without the default interactive selection |
| `--project <regex>` | Only include DerivedData entries whose names match the regex |
| `--older-than <age-or-date>` | Only include entries older than a relative age like `30d`, `2w`, `6m` or an absolute date like `2025-01-01` |
| `--list` | Show matching DerivedData entries without deleting them |
| `--interactive` | Force interactive selection after applying any filters |

### Examples

```bash
nuke derived                  # Choose specific DerivedData entries interactively
nuke derived --yes            # Delete all DerivedData immediately
nuke derived --project 'My.*' # Delete only matching projects
nuke derived --older-than 30d # Delete only older DerivedData entries
nuke derived --list           # List current DerivedData entries
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
