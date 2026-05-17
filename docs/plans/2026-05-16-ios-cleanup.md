# iOS Cleanup Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add safe Xcode and iOS cleanup commands for archives, device support files, and module cache.

**Architecture:** Reuse the existing `internal.Run()` flow for any cleanup target that is a single directory. Add three new `internal.Target` values in `cmd/root.go`, wire three thin Cobra subcommands in `cmd/`, and extend command and README tests to cover the new surface area.

**Tech Stack:** Go, Cobra, table-free standard library tests

---

### Task 1: Add failing command-level tests for new cleanup targets

**Files:**
- Modify: `cmd/cmd_test.go`

**Step 1: Write the failing test**

Add tests that assert:

- `archives`, `device-support`, and `module-cache` are registered subcommands
- each help output contains the expected Xcode/iOS wording
- one new command performs cleanup against an overridden temp directory target

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'Test(Archives|DeviceSupport|ModuleCache)' -v`
Expected: FAIL because the commands do not exist yet.

**Step 3: Write minimal implementation**

Add new command files and target definitions so the tests can pass using the existing cleaner flow.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'Test(Archives|DeviceSupport|ModuleCache)' -v`
Expected: PASS.

### Task 2: Wire the new commands into the CLI

**Files:**
- Modify: `cmd/root.go`
- Create: `cmd/archives.go`
- Create: `cmd/device_support.go`
- Create: `cmd/module_cache.go`

**Step 1: Write the failing test**

This is covered by Task 1.

**Step 2: Run test to verify it fails**

This is covered by Task 1.

**Step 3: Write minimal implementation**

Define:

- `ArchivesTarget`
- `DeviceSupportTarget`
- `ModuleCacheTarget`

and three Cobra commands that call `runTarget(...)`.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'Test(Archives|DeviceSupport|ModuleCache)' -v`
Expected: PASS.

### Task 3: Update user-facing documentation

**Files:**
- Modify: `README.md`

**Step 1: Write the failing test**

No automated README test exists. Use a focused manual diff review after the code is green.

**Step 2: Update the documentation**

Add the new commands to the usage section and briefly describe what each target cleans.

**Step 3: Verify documentation matches behavior**

Run: `go test ./... -v`
Expected: PASS, then inspect the README diff for command names and paths.

### Task 4: Full verification

**Files:**
- Modify: `cmd/cmd_test.go`
- Modify: `README.md`
- Modify: `cmd/root.go`
- Create: `cmd/archives.go`
- Create: `cmd/device_support.go`
- Create: `cmd/module_cache.go`

**Step 1: Run the focused command tests**

Run: `go test ./cmd -run 'Test(Archives|DeviceSupport|ModuleCache)' -v`
Expected: PASS.

**Step 2: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS.

**Step 3: Run vet**

Run: `go vet ./...`
Expected: PASS.
