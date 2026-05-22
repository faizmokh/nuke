# nuke

A Go CLI for cleaning Xcode and iOS development caches on macOS.

Interactive terminal sessions use an enhanced inline UI for selection, summaries, and progress where it helps, while non-interactive runs keep plain text output suitable for scripts.

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
nuke archives   # Clean ~/Library/Developer/Xcode/Archives
nuke device-support # Clean ~/Library/Developer/Xcode/iOS DeviceSupport
nuke module-cache # Clean ~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex
nuke simulators # Clean unavailable CoreSimulator devices
nuke all        # Clean DerivedData and SwiftPM caches
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
nuke derived --interactive    # Force the inline interactive picker
nuke derived --yes            # Delete all DerivedData immediately
nuke derived --project 'My.*' # Delete only matching projects
nuke derived --older-than 30d # Delete only older DerivedData entries
nuke derived --list           # List current DerivedData entries
nuke archives --dry-run       # Preview reclaimable Xcode archive space
nuke device-support --yes     # Delete cached device support files immediately
nuke module-cache             # Clean the Xcode module cache
nuke simulators --dry-run     # Preview unavailable simulators before deleting them
nuke all --dry-run    # Preview what would be deleted
```

### Interactive UX

- `nuke derived` renders immediately in interactive terminals and fills in DerivedData sizes as scanning completes.
- Styled summaries and confirmations are shown only when stdin and stdout are attached to a terminal.
- Piped or scripted usage keeps plain text output.

## Build

```bash
go build -o nuke .
```

## Test

```bash
go test ./... -v
```
