# Terminal UI Design

## Goal

Improve `nuke`'s appearance and interactivity without changing its command structure or breaking scriptable output.

## Scope

This work keeps the existing subcommands and flags, but upgrades interactive terminal sessions with a lightweight terminal UI.

The scope includes:

- a Bubble Tea based interactive picker for `nuke derived`
- styled inline summaries, confirmations, and progress states for interactive terminal use
- plain text fallback for non-interactive sessions and scripted usage
- progressive DerivedData scanning so the UI appears immediately while metadata loads in the background

## Non-Goals

- no full-screen application shell
- no alternate screen mode
- no command renames or flag changes
- no behavior changes to what gets deleted

## UX Direction

The CLI should remain command-first and fast. Interactive enhancements should feel native to a terminal session, not like a separate application.

### Interactive Terminal Behavior

- render inline in the current terminal
- preserve normal shell scrollback
- use styled layouts, colors, and key hints where helpful
- avoid taking over the whole screen

### Non-Interactive Behavior

- preserve plain text output when stdin or stdout is not a terminal
- preserve existing automation-friendly behavior for `--yes`, `--dry-run`, and list-style output

## Command Flows

### `nuke derived`

- read top-level DerivedData entries immediately
- show the picker right away with a loading state
- progressively fill in row metadata such as size and last activity
- allow browsing and selection while scanning continues
- require scanning to finish before final confirmation and deletion
- show inline delete progress and a completion summary

### Other Cleanup Commands

- keep their existing logic
- replace raw prompts and plain progress output with styled summaries, confirmations, and progress where a terminal is interactive
- retain plain text fallback outside TTY mode

## Architecture

- keep `cmd/` focused on Cobra wiring and flag handling
- keep cleanup logic in `internal/`
- add `internal/tui` as a presentation layer for terminal-specific interaction and rendering
- let commands choose TUI or plain mode based on terminal capability and flags

## Progressive DerivedData Scanning

The current delay in `nuke derived` comes from walking every directory before any UI can render. The improved flow should split scanning into two phases:

1. enumerate and sort top-level entries
2. compute per-entry stats in the background and stream updates into the UI

This changes the perceived performance even if total scan time stays similar.

## Filtering Behavior

- `--project` can filter by name before background stat work starts
- `--older-than` depends on last-activity metadata, so rows may become eligible progressively as stats finish
- `--list` and `--dry-run` should keep simple non-TUI output

## Risks

- terminal-only rendering must not leak into tests or pipeline usage
- progressive filtering for `--older-than` must remain understandable to users
- the Bubble Tea layer should stay thin and not replace the core cleanup logic
