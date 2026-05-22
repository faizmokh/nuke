# Terminal UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve `nuke`'s interactive appearance while preserving scriptable Cobra commands, and make `nuke derived` feel fast by rendering immediately and progressively filling in scan results.

**Architecture:** Keep Cobra and the existing cleanup logic intact. Add a thin `internal/tui` presentation layer plus a progressive DerivedData scanning API in `internal/derived.go`; commands decide between plain text fallback and TUI mode based on terminal capability and flags.

**Tech Stack:** Go, Cobra, Bubble Tea, Bubbles, Lip Gloss, `golang.org/x/term`

---

### Task 1: Add terminal UI dependencies and terminal detection

**Files:**
- Modify: `go.mod`
- Create: `internal/tui/terminal.go`
- Create: `internal/tui/terminal_test.go`

**Step 1: Write the failing test**

Cover non-`*os.File` readers and writers returning `false` from `IsInteractiveTerminal`.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestIsInteractiveTerminal -v`

Expected: FAIL because the package or helper does not exist yet.

**Step 3: Write minimal implementation**

Add Bubble Tea-related dependencies and implement a helper that returns `true` only when both input and output are terminals.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestIsInteractiveTerminal -v`

Expected: PASS

**Step 5: Commit**

`git add go.mod go.sum internal/tui/terminal.go internal/tui/terminal_test.go`

`git commit -m "feat: add terminal ui detection"`

### Task 2: Refactor DerivedData scanning into a progressive pipeline

**Files:**
- Modify: `internal/derived.go`
- Modify: `internal/derived_test.go`

**Step 1: Write the failing test**

Add tests for a progressive scan API that emits all top-level entries in stable name order, reports incremental updates as stats complete, and supports reconstructing the current `ScanDerived` result.

**Step 2: Run test to verify it fails**

Run: `go test ./internal -run 'TestScanDerived|TestScanDerivedProgressive' -v`

Expected: FAIL because the progressive scan API does not exist yet.

**Step 3: Write minimal implementation**

Introduce a streaming API, such as `ScanDerivedProgressively`, and keep `ScanDerived` as a wrapper over the progressive path.

**Step 4: Run test to verify it passes**

Run: `go test ./internal -run 'TestScanDerived|TestScanDerivedProgressive' -v`

Expected: PASS

**Step 5: Commit**

`git add internal/derived.go internal/derived_test.go`

`git commit -m "refactor: add progressive derived scanning"`

### Task 3: Build the DerivedData Bubble Tea picker

**Files:**
- Create: `internal/tui/derived_model.go`
- Create: `internal/tui/derived_model_test.go`

**Step 1: Write the failing test**

Cover initial loading state, row updates as scan messages arrive, selection toggling, confirm being blocked until scan completes, and cancel returning no selection.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestDerivedModel -v`

Expected: FAIL because the model does not exist yet.

**Step 3: Write minimal implementation**

Implement a Bubble Tea model that shows a header with scan progress, selectable rows, loading placeholders, and compact key help without using the alternate screen.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestDerivedModel -v`

Expected: PASS

**Step 5: Commit**

`git add internal/tui/derived_model.go internal/tui/derived_model_test.go`

`git commit -m "feat: add derived tui picker"`

### Task 4: Wire `cmd/derived.go` to the new UI without breaking flags

**Files:**
- Modify: `cmd/derived.go`
- Modify: `cmd/cmd_test.go`

**Step 1: Write the failing test**

Cover TUI routing for interactive terminals, and ensure `--yes`, `--dry-run`, and `--list` keep non-TUI behavior.

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestDerivedCommand.*' -v`

Expected: FAIL because the command still uses the old interaction path.

**Step 3: Write minimal implementation**

Add injectable terminal detection and picker execution so `derived` can choose the inline TUI path.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestDerivedCommand.*' -v`

Expected: PASS

**Step 5: Commit**

`git add cmd/derived.go cmd/cmd_test.go`

`git commit -m "feat: use tui for interactive derived command"`

### Task 5: Add shared styled summaries and progress for the other commands

**Files:**
- Create: `internal/tui/summary.go`
- Create: `internal/tui/progress.go`
- Modify: `internal/cleaner.go`
- Modify: `cmd/root.go`
- Modify: `cmd/simulators.go`

**Step 1: Write the failing test**

Add tests around summary and progress rendering selection logic while keeping non-interactive output unchanged.

**Step 2: Run test to verify it fails**

Run: `go test ./internal ./cmd -run 'TestRun|TestSimulators' -v`

Expected: FAIL because the presentation helpers do not exist yet.

**Step 3: Write minimal implementation**

Add reusable helpers for styled preview cards, styled confirmations, and nicer live progress in interactive terminals.

**Step 4: Run test to verify it passes**

Run: `go test ./internal ./cmd -run 'TestRun|TestSimulators' -v`

Expected: PASS

**Step 5: Commit**

`git add internal/tui/summary.go internal/tui/progress.go internal/cleaner.go cmd/root.go cmd/simulators.go`

`git commit -m "feat: add styled terminal summaries and progress"`

### Task 6: Final verification and docs

**Files:**
- Modify: `README.md`

**Step 1: Update docs**

Describe the enhanced interactive terminal experience and the plain-text fallback behavior.

**Step 2: Run tests**

Run: `go test ./... -v`

Expected: PASS

**Step 3: Run vet**

Run: `go vet ./...`

Expected: PASS

**Step 4: Manual verification**

Run and inspect:

- `nuke derived`
- `nuke derived --project 'My.*'`
- `nuke derived --older-than 30d`
- `nuke derived --list`
- `nuke all`
- `nuke simulators --dry-run`

**Step 5: Commit**

`git add README.md`

`git commit -m "docs: describe enhanced interactive ux"`
