# iOS Cleanup Expansion Design

## Goal

Expand `nuke` with additional Xcode and iOS development cleanup commands while keeping the tool narrowly focused on safe, explicit disk cleanup.

## Scope

The first implementation wave adds three new commands:

- `nuke archives`
- `nuke device-support`
- `nuke module-cache`

Each command maps to a single well-known filesystem target and reuses the existing command behavior:

- print a size and item summary
- support `--yes` and `--dry-run`
- show progress during deletion
- report freed space after deletion

## Target Paths

- `archives` -> `~/Library/Developer/Xcode/Archives`
- `device-support` -> `~/Library/Developer/Xcode/iOS DeviceSupport`
- `module-cache` -> `~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex`

## Non-Goals

- No broad `ios` aggregate command yet
- No simulator cleanup in this wave, because blanket simulator deletion is riskier and needs more precise semantics
- No new diagnostics or doctor-style reporting yet

## Implementation Notes

The existing generic cleaner flow already supports fixed-directory targets, so these commands should be thin Cobra wrappers plus shared target definitions. Tests should cover command registration, help output, and at least one end-to-end cleanup path for a new command to prove the generic integration works.
