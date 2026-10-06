# AGENTS.md

## Project

`nuke` is a Go CLI for cleaning Xcode and iOS development caches on macOS.
Module: `github.com/faizmokh/nuke`.

## Commands

| Command | Target |
|---------|--------|
| `spm download` | Resolve and fetch existing Xcode/Swift package dependencies with live output |
| `doctor` | Read-only environment diagnostics: developer tools, SDKs, runtimes, cache access, disk space |
| `status` | Read-only size/item overview of all filesystem targets and unavailable simulator counts |
| `clean derived` | `~/Library/Developer/Xcode/DerivedData` |
| `clean spm` | `~/Library/Caches/org.swift.swiftpm` |
| `clean archives` | `~/Library/Developer/Xcode/Archives` |
| `clean device-support` | `~/Library/Developer/Xcode/iOS DeviceSupport` |
| `clean module-cache` | `~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex` |
| `clean simulators` | Unavailable CoreSimulator devices, through `xcrun simctl` |
| `clean caches` | DerivedData and SPM caches only |

Cleanup flags (under `clean`): `--yes` / `-y` skips entry selection and confirmation; `--dry-run` previews without
reading stdin or deleting. DerivedData also supports `--all`, `--project <regex>`,
`--older-than <positive age or YYYY-MM-DD>`, `--list`, `--select`, `--current`, and `--build-only`.
`--current` matches canonical paths from Xcode entry metadata, not project names;
ambiguous discovery or malformed metadata must fail before deletion.
The shared `--derived-data <path>` explicitly overrides the DerivedData root for
`clean derived`, `clean caches`, `status`, `doctor`, and `clean module-cache`; no automatic preferences lookup.
`--list` and `--dry-run` always bypass interactive selection and confirmation.

Bare `nuke` shows help. Bare `nuke clean` opens an inline single-choice target menu in terminals; nonterminal usage and bare `clean --yes` / `clean --dry-run` require an explicit target without reading stdin. Old top-level cleanup commands are removed.

`clean archives --trash` moves confirmed top-level entries to `~/.Trash` on macOS, using directory-anchored, exclusive renames. Collisions get unique names; move failures (including cross-volume moves) have no permanent-deletion fallback. Dry runs never create Trash. Report estimated bytes moved, never freed; disk space is reclaimed only after emptying Trash. `--trash` is archive-only.

## Architecture

- `cmd/root.go`: independent Cobra command construction with `NewRootCommand`.
- `internal/tui/target_menu.go`: inline cleanup target selection.
- `cmd/cleanup.go`: shared filesystem cleanup orchestration, plain/styled reporting
  and cancellable confirmation.
- `cmd/doctor.go`, `internal/doctor*.go`: read-only diagnostics; five-second tool
  timeouts, cancellation, root access checks without write probes, and disk space
  at existing ancestors of configured cache paths. Errors exit 1; warnings exit 0.
- `cmd/status.go`: read-only overview; reuse the DerivedData module-cache size and
  exclude that nested row from the total. Missing targets are normal, failures
  aggregate, and status never reads stdin or deletes.
- `cmd/packages.go`, `internal/packages.go`, `internal/package_tool.go`, `internal/package_process*.go`: nearest package-container discovery, native resolution with locked versions when available, serialized/redacted streaming logs, bounded network retries, cancellation of subprocess groups, and per-container OS locks. Never reset caches or bypass dependency verification.
- `cmd/derived.go`, `cmd/simulators.go`: command-specific options and workflows.
- `internal/current.go`: nearest workspace/project discovery and metadata matching
  before traversal; in-process XML parsing and plutil for binary metadata.
- `internal/scan.go`: `ScanTarget`, per-invocation `ScanPlan`, name filtering before
  traversal, bounded workers, serialized progressive updates, and scan errors.
- `internal/build.go`: build-only scan/deletion snapshots for exactly
  `Build/Products` and `Build/Intermediates.noindex`; preserve other data and skip
  symlinked parents/outputs. Sizes and age describe only these outputs.
- `internal/trash*.go`: recoverable archive moves through the shared plan revalidation.
- `internal/delete.go`: `DeletePlan`, sequential deletion, containment and identity
  revalidation, per-entry results, and aggregated failures.
- `internal/derived.go`: entry metadata, age parsing, plain tables and selection;
  older scan/deletion helpers remain compatibility adapters.
- `internal/tui/`: inline Bubble Tea picker, width/height handling, summaries and
  throttled progress. Rows and selection use entry paths rather than scan indexes.
- `internal/simulators.go`: context-aware subprocesses and unavailable-device parsing.
- `internal/cleaner.go`, `size.go`: compatibility helpers and size formatting.

## Important Behavior

- Build-only mode preserves project roots and all indexes, package checkouts,
  metadata, logs, and unrelated Build children. Revalidate nested output and parent
  identities, count successful output bytes, and aggregate partial failures.
- Delete target contents, preserving target directories. Delete only top-level
  entries in the confirmed plan; newly discovered siblings are never swept in.
- Scan each eligible entry once using `filepath.WalkDir`, without following symlinks.
  At most four workers, capped by CPU and entry count; final entries sort by name.
- DerivedData age uses the latest descendant file modification time, with the
  entry's modification time as fallback for empty trees.
- Failed scans are visible and cannot be selected for deletion. Other entries and
  independent targets may proceed; partial failures return an error (exit code 1).
- Reuse scan estimates for deletion reporting. Count bytes only for entries whose
  removal succeeds; these are logical-size estimates, not physical disk measurements.
- Revalidate root and entry identity before removal. `os.Root` confines removal to
  the scanned directory. No recursive rescan occurs at deletion time; changes inside
  a still-valid directory can make the estimate stale.
- Context cancellation stops new scan jobs, is checked during walks and between
  deletions, interrupts file-based confirmation reads and simulator subprocesses,
  and shuts down picker scanning when it closes. An in-progress RemoveAll finishes
  before cancellation is checked again.
- Interactive UI preserves scrollback; piped output has no terminal progress controls.
  Progress renders at most 10 times per second, with an immediate final delete update.

## Development

```bash
go build -o nuke .
go install .
go test ./...
go test -race ./...
go vet ./...
go test ./internal -run '^$' -bench '^BenchmarkScan$' -benchmem -benchtime=3x -count=2
```

Benchmarks build temporary fixtures outside timed sections and never clean user
caches. See `docs/performance.md` for baseline methodology and measured results.
Dependencies include Cobra, Bubble Tea/Bubbles, Lip Gloss, terminal/ANSI helpers,
and cancelreader. Keep deletion sequential; benchmark before changing scan concurrency.
