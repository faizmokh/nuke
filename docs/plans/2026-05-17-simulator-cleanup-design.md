# Simulator Cleanup Design

## Goal

Add a safe simulator cleanup command that removes only unavailable CoreSimulator devices.

## Scope

This wave adds one new command:

- `nuke simulators`

The command should:

- inspect simulator devices via `xcrun simctl list devices --json`
- identify devices where `isAvailable` is `false`
- print a preview of what would be removed
- support `--dry-run` and `--yes`
- delete unavailable devices via `xcrun simctl delete unavailable`

## Safety Boundary

The command must not delete available simulators. It should only remove devices that CoreSimulator already marks unavailable.

## UX

- If no unavailable simulators exist, print a no-op message and exit successfully.
- If unavailable simulators exist, print their names and runtimes, plus a count summary.
- `--dry-run` should stop after previewing the unavailable devices.
- `--yes` should skip confirmation.

## Implementation Notes

Simulator cleanup should not reuse the generic directory-based cleaner flow because unavailable devices are determined by `simctl` state, not by a single fixed filesystem target. The implementation should live in a simulator-specific path with an injectable runner so command tests can verify behavior without depending on real simulator state.
