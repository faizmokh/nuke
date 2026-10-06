# nuke

A Go CLI for cleaning Xcode and iOS development caches and preparing Swift package dependencies on macOS.

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
nuke clean                                 # Choose a cleanup target in an interactive terminal
nuke doctor                                # Diagnose the iOS development environment without making changes
nuke status                                # Read-only overview of cache sizes and unavailable simulators
nuke clean derived                         # Interactively clean ~/Library/Developer/Xcode/DerivedData
nuke clean spm                             # Clean ~/Library/Caches/org.swift.swiftpm
nuke clean archives                        # Clean ~/Library/Developer/Xcode/Archives
nuke clean device-support                  # Clean ~/Library/Developer/Xcode/iOS DeviceSupport
nuke clean module-cache                    # Clean ~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex
nuke clean simulators                      # Clean unavailable CoreSimulator devices
nuke clean caches                          # Clean DerivedData and SwiftPM caches
```

Run `nuke` for a short command overview. `nuke clean` opens a single-choice menu: use arrows or `j`/`k`, Enter to continue, and `q`/Esc to cancel. Scripts and bare `clean --yes` or `clean --dry-run` require an explicit target. Existing top-level cleanup commands have moved under `clean`; `all` is now `clean caches`, and `--interactive` is now `--select`.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--yes` | `-y` | Skip entry selection and confirmation (cleanup only) |
| `--dry-run` | | Show what would be deleted without deleting (cleanup only) |
| `--derived-data <path>` | | Override the DerivedData root for `clean derived`, `clean caches`, `clean module-cache`, `status`, and `doctor` |
| `--version` | | Print version |

### Download Swift package dependencies

Run from your app repository or a subfolder:

```bash
nuke spm download
nuke spm download --workspace App.xcworkspace
nuke spm download --project App.xcodeproj --scheme App
nuke spm download --package Packages/Core
nuke spm download --verbose
```

The nearest enclosing setup is selected, with workspaces preferred over projects,
then standalone `Package.swift` packages. Multiple equally eligible containers
open an inline picker in terminals; scripts must specify a path. A workspace
resolves its package graph through Xcode, including its referenced projects and
local packages. Workspaces and custom DerivedData paths require a scheme: a sole scheme is
selected automatically, multiple schemes open a picker, and scripts with multiple
schemes must specify `--scheme`. Project-only resolution can omit a scheme.
`--scheme` explicitly chooses an Xcode scheme. Generated projects must exist first (run Tuist/XcodeGen yourself).
`--project`, `--workspace`, and `--package` are mutually exclusive. `--package`
also accepts a `Package.swift` file. Standalone packages reject `--scheme` and
`--derived-data`; Xcode downloads accept the shared `--derived-data` override with automatic scheme discovery when needed.

Downloads use the selected developer toolchain via `xcrun`: Xcode's
`-resolvePackageDependencies` for apps and `swift package resolve` for standalone
packages. Existing `Package.resolved` versions are enforced. Missing lockfiles
are announced before initial resolution, which may create a lockfile. Incompatible
lockfiles fail with guidance to resolve and review them in Xcode/SwiftPM. The
command does not add dependencies, upgrade all versions, reset caches, or build
app targets. A successful no-dependency or already-cached run also reports
“Dependencies ready”; package counts and cache hits are shown only when the tool
reports them.

Output streams fetching, resolution, checkout, artifact activity, warnings, and
errors, with elapsed-time updates every ten seconds. `--verbose` includes other
tool output. Output uses plain lines suitable for terminals and logs; no invented
percentage is shown. Recent output is retained for failures. Common credential
URLs, token query parameters, and authorization/password fields are redacted,
but review verbose logs before sharing them because tools may print other private
project information.

Private dependencies use credentials already available to the underlying tools;
child processes do not read stdin or prompt for credentials. Transient network
failures retry up to twice; authentication, checksum, and lockfile failures are
not retried. Ctrl+C stops the subprocess group. Concurrent `nuke` downloads for
the same canonical container are blocked with an OS lock outside the repository;
this lock does not coordinate with Xcode or other package-manager processes.
No cache cleanup or artifact-validation bypass is performed on failure.

### Recoverable archive cleanup

```bash
nuke clean archives --trash --dry-run  # Preview archives to move
nuke clean archives --trash            # Confirm and move to Trash
nuke clean archives --trash --yes      # Move without confirmation
```

`--trash` is available only for `clean archives` on macOS. It moves the confirmed
snapshot's top-level entries into your user Trash (`~/.Trash`), preserving the
Archives directory and any new siblings. Existing Trash items are never
overwritten; duplicate names receive a unique suffix. Open Trash in Finder to
recover moved entries by moving them back to the Archives directory.

**Disk space is reclaimed only after emptying Trash.** Reported bytes are the
estimated size moved, not space freed. Dry runs never create Trash or move files.
Without `--trash`, archive cleanup continues to permanently delete entries.

Moves across volumes or other move failures leave the affected entries in place
and return an error; nuke never falls back to permanent deletion. Other confirmed
entries can still move successfully. No automatic emptying of Trash occurs.

### DerivedData Flags

| Flag | Description |
|------|-------------|
| `--build-only` | Remove only `Build/Products` and `Build/Intermediates.noindex` from matching projects |
| `--current` | Match the nearest enclosing Xcode workspace/project by its recorded filesystem path |
| `--all` | Skip DerivedData entry selection; still ask for confirmation |
| `--project <regex>` | Only include DerivedData entries whose names match the regex |
| `--older-than <age-or-date>` | Only include entries older than a relative age like `30d`, `2w`, `6m` or an absolute date like `2025-01-01` |
| `--list` | Show matching DerivedData entries without deleting them |
| `--select` | Force interactive selection after applying any filters |

### Examples

```bash
nuke clean derived                         # Choose specific DerivedData entries interactively
nuke clean derived --select                # Force the inline interactive picker
nuke clean derived --yes                   # Delete all DerivedData immediately
nuke clean derived --project 'My.*'        # Delete only matching projects
nuke clean derived --older-than 30d        # Delete only older DerivedData entries
nuke clean derived --list                  # List current DerivedData entries
nuke clean archives --dry-run              # Preview reclaimable Xcode archive space
nuke clean device-support --yes            # Delete cached device support files immediately
nuke clean module-cache                    # Clean the Xcode module cache
nuke clean simulators --dry-run            # Preview unavailable simulators before deleting them
nuke clean caches --dry-run                # Preview what would be deleted
```

### Clean the current project

```bash
nuke clean derived --current --dry-run
nuke clean derived --current
nuke clean derived --current --older-than 30d
nuke clean derived --current --derived-data ~/BuildCaches/DerivedData
nuke status --derived-data ~/BuildCaches/DerivedData
```

`--current` searches the current directory and its parents for the nearest Xcode
workspace or project. A workspace takes precedence over projects beside it.
Multiple workspaces (or multiple projects without a workspace) produce an error;
run from inside the intended `.xcworkspace`/`.xcodeproj` bundle to disambiguate.

Entries match the canonical workspace/project path recorded in their `info.plist`,
so equally named projects in different directories remain separate. Entries
without matching metadata are skipped, including module caches. XML metadata is
read in-process; binary metadata uses macOS `plutil`. Invalid metadata stops the
operation before deletion. No name-based fallback is used.

`--current` supports age filters, previews, and optional interactive selection;
it cannot be combined with `--project` or `--all`. Matching entries still require
confirmation unless `--yes` is set.

`--derived-data` explicitly sets the **root containing DerivedData entries**,
including relative paths or `~/` paths. It does not automatically read Xcode's
location preferences. The default location remains unchanged. `clean module-cache`
uses the override's `ModuleCache.noindex` directory; `clean caches` still targets only
DerivedData and SwiftPM. Custom cache layouts without Xcode's per-entry metadata
can use the existing `--project` filter instead of `--current`.

### Build-only cleanup

```bash
nuke clean derived --current --build-only --dry-run
nuke clean derived --current --build-only
nuke clean derived --build-only --project '^MyApp-' --yes
nuke clean derived --build-only --all --older-than 30d
```

Build-only cleanup removes exactly `Build/Products` and
`Build/Intermediates.noindex` inside matching DerivedData project directories.
Project roots, the `Build` parent, indexes (`Index.noindex`), package checkouts
(`SourcePackages`), logs, metadata, module caches, and other Build children remain.
Symlinked projects, Build directories, and output directories are skipped.

The mode supports current-project matching, regex/age filters, custom roots,
interactive selection, listing, and dry runs. Previews and confirmation show only
build-output sizes. Age filtering uses the latest modification time in those
outputs, rather than preserved indexes or checkouts. Entries without eligible
outputs are omitted. Unknown/custom build-directory layouts are left untouched.

Deletion revalidates each output directory and its project/Build parents. Only
confirmed outputs are removed, and bytes are counted per successful removal.
If part of a project's cleanup fails, the command reports the partial failure and
returns exit code 1; its completed-item count includes only fully cleaned projects.

### Environment diagnostics

```bash
nuke doctor
nuke doctor --derived-data ~/BuildCaches/DerivedData
```

Checks the effective developer directory (including `DEVELOPER_DIR` overrides),
Xcode version, iOS device/simulator SDKs, `simctl`, available iOS simulator runtimes,
cache-root read/write/search permissions, and free disk space at configured cache
locations. Missing cache directories are normal. Directory access checks cover
roots; nested files can have different permissions.

Each finding has an OK, INFO, WARN, or ERROR label and actionable guidance where
needed. Missing simulator runtimes and less than 10 GiB available are warnings;
the disk threshold is a practical heuristic. Tool, SDK, permission, or disk-query
failures return exit code 1. Warnings alone return 0. Tool checks time out after
five seconds and respond to Ctrl-C.

Doctor never prompts, reads stdin, installs components, accepts licenses, writes
permission-probe files, or deletes data. Cleanup flags are available only under `clean`. Plain output is suitable for pipelines; terminals show a
summary card.

### Cache status

```bash
nuke status
```

Shows estimated size, top-level item count, and availability for DerivedData,
SwiftPM caches, archives, device support, and module caches. It also reports the
number of unavailable simulator devices without measuring their disk usage.

ModuleCache is normally inside DerivedData: its scan size is reused and its row
is marked as included, so the filesystem total counts it once. Sizes are logical
estimates, and partial scans produce a clearly labelled known-size subtotal.
Missing cache directories are normal; scan or simulator-tool errors are reported
while other targets remain visible, with exit code 1.

The command never reads stdin, prompts, or deletes anything. Cleanup flags are available only under `clean`. Terminal sessions get a summary
card; pipelines get a plain table.

### Interactive UX

- `nuke clean derived` renders immediately in interactive terminals and fills in DerivedData sizes as scanning completes.
- Use arrows or `j`/`k` to move, Space to select, `a` to select all, `n` to select none, Enter to continue, and `q`/Esc to cancel.
- The picker scrolls to fit the terminal and shows selected count and estimated size. Entries can be selected while scanning; failed entries become unavailable.
- `--list` and `--dry-run` bypass selection and confirmation, even with `--select`.
- Styled summaries and confirmations are shown only when stdin and stdout are attached to a terminal.
- Piped or scripted usage keeps plain text output without animated progress or carriage returns.

### Cleanup behavior and performance

Each run scans eligible entries once and reuses that snapshot for preview, confirmation, and deletion. Project regexes are validated and applied before recursive scanning. Age filters use the latest descendant file modification time; relative ages must be positive. Scans use up to four workers, while deletion stays sequential.

Only entries in the confirmed snapshot are removed. Target directories are preserved, symlink destinations are not scanned, and entries replaced or changed at the top level since scanning are skipped. Unreadable entries and deletion failures are reported; partial failures return exit code 1, including with `nuke clean caches`. Independent targets can still finish.

Reported sizes are logical-size estimates from the scan, counted only after successful removal. Changes inside a directory between scanning and deletion can make its estimate stale. Ctrl-C cancels scanning, confirmation, or simulator subprocesses; filesystem deletion checks cancellation between entries, after an in-progress removal finishes.

`nuke clean caches` continues to clean **DerivedData and SwiftPM only**. Archives, device support, module caches, and unavailable simulators remain explicit commands.

See [performance measurements](docs/performance.md) for reproducible benchmarks on temporary macOS cache fixtures.

## Build

```bash
go build -o nuke .
```

## Test

```bash
go test ./... -v
go test -race ./...
go vet ./...
```
