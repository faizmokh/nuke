# Simulator Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `nuke simulators` to preview and delete unavailable CoreSimulator devices safely.

**Architecture:** Introduce simulator-specific logic in `internal/` that shells out to `xcrun simctl` through an injected runner. Keep the Cobra command thin in `cmd/`, using package-level function variables so tests can stub listing and deletion without invoking real simulator state.

**Tech Stack:** Go, Cobra, standard library JSON parsing, standard library tests

---

### Task 1: Add failing simulator command tests

**Files:**
- Modify: `cmd/cmd_test.go`

**Step 1: Write the failing test**

Add tests that assert:

- `simulators` is a registered command
- help output mentions unavailable simulators
- `--dry-run` lists unavailable simulators without deleting
- `--yes` triggers deletion without confirmation

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestSimulators' -v`
Expected: FAIL because the command and simulator hooks do not exist yet.

**Step 3: Write minimal implementation**

Add a new command file and simulator function hooks so the tests can pass.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestSimulators' -v`
Expected: PASS.

### Task 2: Add failing internal simulator tests

**Files:**
- Create: `internal/simulators_test.go`
- Create: `internal/simulators.go`

**Step 1: Write the failing test**

Add tests that assert:

- unavailable devices are extracted from `simctl list devices --json`
- runtime identifiers are preserved in returned results
- the delete path invokes `xcrun simctl delete unavailable`

**Step 2: Run test to verify it fails**

Run: `go test ./internal -run 'Test(ListUnavailableSimulators|DeleteUnavailableSimulators)' -v`
Expected: FAIL because the simulator functions do not exist yet.

**Step 3: Write minimal implementation**

Parse the JSON into a small struct, collect only unavailable devices, and wrap the delete command in a helper that accepts an injected runner.

**Step 4: Run test to verify it passes**

Run: `go test ./internal -run 'Test(ListUnavailableSimulators|DeleteUnavailableSimulators)' -v`
Expected: PASS.

### Task 3: Wire command behavior and docs

**Files:**
- Create: `cmd/simulators.go`
- Modify: `README.md`
- Modify: `cmd/root.go`

**Step 1: Write the failing test**

Use the command tests from Task 1.

**Step 2: Run test to verify it fails**

Use the command test command from Task 1.

**Step 3: Write minimal implementation**

Implement `nuke simulators` to:

- list unavailable simulators
- print them in a simple text preview
- stop on `--dry-run`
- confirm unless `--yes`
- delete unavailable simulators through the internal helper

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestSimulators' -v`
Expected: PASS.

### Task 4: Full verification

**Files:**
- Modify: `cmd/cmd_test.go`
- Modify: `cmd/root.go`
- Modify: `README.md`
- Create: `cmd/simulators.go`
- Create: `internal/simulators.go`
- Create: `internal/simulators_test.go`

**Step 1: Run focused tests**

Run: `go test ./cmd -run 'TestSimulators' -v`
Expected: PASS.

**Step 2: Run internal simulator tests**

Run: `go test ./internal -run 'Test(ListUnavailableSimulators|DeleteUnavailableSimulators)' -v`
Expected: PASS.

**Step 3: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS.

**Step 4: Run vet**

Run: `go vet ./...`
Expected: PASS.
