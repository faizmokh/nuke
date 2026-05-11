# AGENTS.md

## Project

`nuke` — A Go CLI for cleaning Xcode DerivedData and Swift Package Manager caches on macOS.

**Module:** `github.com/faizmokh/nuke`

## Commands

| Command | Target |
|---------|--------|
| `nuke derived` | `~/Library/Developer/Xcode/DerivedData` |
| `nuke spm` | `~/Library/Caches/org.swift.swiftpm` |
| `nuke all` | Both of the above |

## Flags

- `--yes` / `-y` — skip confirmation prompt
- `--dry-run` — show what would be deleted without deleting

## Project Structure

```
nuke/
├── main.go                # entry point
├── cmd/
│   ├── root.go            # root command, --version, shared helpers
│   ├── derived.go         # nuke derived
│   ├── spm.go             # nuke spm
│   ├── all.go             # nuke all
│   └── cmd_test.go        # command registration and help tests
├── internal/
│   ├── cleaner.go         # scan, confirm, delete, report logic
│   ├── cleaner_test.go    # cleaner unit tests (temp dir fixtures)
│   ├── size.go            # human-readable size formatting
│   └── size_test.go       # size formatting table-driven tests
├── go.mod
└── go.sum
```

## Build & Run

```bash
go build -o nuke .
go install .
```

## Test

```bash
go test ./... -v
```

## Lint

```bash
go vet ./...
```

## Dependencies

- **Cobra** (`github.com/spf13/cobra`) — CLI framework

## Key Implementation Details

- Deletes **contents** of target dirs, not the dirs themselves (Xcode recreates them)
- `internal.Run()` handles the full flow: scan → confirm → delete → report
- `internal.Scan()` calculates total size and item count
- `internal.Nuke()` performs the actual deletion and returns bytes freed
- `internal.HumanSize()` formats bytes as human-readable strings (B, KB, MB, GB, TB)
- Exit code 0 on success, 1 on error
