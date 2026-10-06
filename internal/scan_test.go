package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureTarget(t *testing.T) Target {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"A", "B", "C"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "file"), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return Target{Name: "test", Path: dir}
}

func TestScanFiltersBeforeTraversalAndSerializesUpdates(t *testing.T) {
	target := fixtureTarget(t)
	var traversals atomic.Int32
	active := false
	var updates []ScanUpdate
	plan, err := scanTarget(context.Background(), target, ScanOptions{Project: "^[AB]$", Workers: 4, Activity: true}, func(u ScanUpdate) {
		if active {
			t.Fatal("observer called concurrently")
		}
		active = true
		updates = append(updates, u)
		active = false
	}, func(ctx context.Context, path string, activity bool) DerivedEntry {
		traversals.Add(1)
		if filepath.Base(path) == "C" {
			t.Error("excluded entry was scanned")
		}
		return scanEntry(ctx, path, activity)
	})
	if err != nil {
		t.Fatal(err)
	}
	if traversals.Load() != 2 || len(updates) != 4 || len(plan.Entries) != 2 {
		t.Fatalf("traversals=%d updates=%d entries=%d", traversals.Load(), len(updates), len(plan.Entries))
	}
	if plan.Entries[0].Name != "A" || plan.Entries[1].Name != "B" {
		t.Fatal("plan not sorted")
	}
	// Deletion must reuse the scanned sizes, even if contents change in place.
	if err := os.WriteFile(filepath.Join(target.Path, "A", "file"), []byte("larger content"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := DeletePlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes != 2 || traversals.Load() != 2 {
		t.Fatalf("result=%+v traversals=%d", result, traversals.Load())
	}
	if _, err := os.Stat(filepath.Join(target.Path, "C", "file")); err != nil {
		t.Fatal("excluded entry removed", err)
	}
}

func TestInvalidFiltersValidatedBeforeFilesystem(t *testing.T) {
	for _, opts := range []ScanOptions{{Project: "["}, {OlderThan: "-1d"}, {OlderThan: "0d"}, {OlderThan: "1000000000w"}} {
		_, err := ScanTarget(context.Background(), Target{Path: "/missing"}, opts, nil)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatalf("options=%+v error=%v", opts, err)
		}
	}
}

func TestScanFailuresRemainInPlan(t *testing.T) {
	target := fixtureTarget(t)
	failure := errors.New("unreadable")
	plan, err := scanTarget(context.Background(), target, ScanOptions{}, nil, func(ctx context.Context, path string, activity bool) DerivedEntry {
		if filepath.Base(path) == "B" {
			return DerivedEntry{Name: "B", Path: path, Err: failure}
		}
		return scanEntry(ctx, path, activity)
	})
	if !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	size, count := plan.Summary()
	if size != 2 || count != 2 || len(plan.Entries) != 3 {
		t.Fatalf("size=%d count=%d plan=%+v", size, count, plan)
	}
	selected := plan.Select(plan.Entries)
	result, err := DeletePlan(context.Background(), selected, nil)
	if err != nil || result.Items != 2 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "B")); err != nil {
		t.Fatal("failed entry removed", err)
	}
}

func TestScanCancellationStopsScheduling(t *testing.T) {
	target := fixtureTarget(t)
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	_, err := scanTarget(ctx, target, ScanOptions{Workers: 1}, nil, func(ctx context.Context, path string, activity bool) DerivedEntry {
		calls.Add(1)
		cancel()
		return scanEntry(ctx, path, activity)
	})
	if !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
	_, err = ScanTarget(ctx, target, ScanOptions{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestScanSymlinksAreNotFollowed(t *testing.T) {
	target := fixtureTarget(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "large"), make([]byte, 4096), 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(target.Path, "link"), filepath.Join(target.Path, "A", "nested-link")} {
		if err := os.Symlink(outside, path); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := ScanTarget(context.Background(), target, ScanOptions{Activity: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	size, _ := plan.Summary()
	if size != int64(3+2*len(outside)) {
		t.Fatalf("size=%d follows symlink or ignores link metadata", size)
	}
	if _, err := DeletePlan(context.Background(), plan, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "large")); err != nil {
		t.Fatal("symlink destination removed", err)
	}
}

func TestDeletePlanSkipsChangedMissingAndOutsideEntries(t *testing.T) {
	target := fixtureTarget(t)
	plan, err := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the old inode alive to prevent accidental inode reuse.
	if err := os.Rename(filepath.Join(target.Path, "A"), filepath.Join(target.Path, "old-A")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(target.Path, "A"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(target.Path, "B")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	plan.Entries = append(plan.Entries, DerivedEntry{Name: "outside", Path: outside})
	result, err := DeletePlan(context.Background(), plan, nil)
	if err == nil || result.Items != 1 || result.Bytes != 1 || len(result.Entries) != 4 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "A")); err != nil {
		t.Fatal("replacement removed", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("outside removed", err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "old-A")); err != nil {
		t.Fatal("unconfirmed entry removed", err)
	}
}

func TestDeletePlanPartialFailureAndCancellation(t *testing.T) {
	target := fixtureTarget(t)
	plan, err := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("permission denied")
	result, err := deletePlan(context.Background(), plan, nil, func(root *os.Root, name string) error {
		if name == "B" {
			return failure
		}
		return root.RemoveAll(name)
	})
	if !errors.Is(err, failure) || result.Items != 2 || result.Bytes != 2 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "B")); err != nil {
		t.Fatal(err)
	}
	target = fixtureTarget(t)
	plan, err = ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, err = DeletePlan(ctx, plan, func(current, total int) { cancel() })
	if !errors.Is(err, context.Canceled) || result.Items != 1 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestDeletePlanRejectsReplacedTarget(t *testing.T) {
	target := fixtureTarget(t)
	plan, err := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target.Path, target.Path+"-old"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(target.Path + "-old") })
	if err := os.Mkdir(target.Path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = DeletePlan(context.Background(), plan, nil); err == nil {
		t.Fatal("replaced target accepted")
	}
}

func TestAgeUsesLatestDescendantModification(t *testing.T) {
	target := fixtureTarget(t)
	old := time.Now().Add(-90 * 24 * time.Hour)
	for _, name := range []string{"A", "B", "C"} {
		if err := os.Chtimes(filepath.Join(target.Path, name, "file"), old, old); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	if err := os.Chtimes(filepath.Join(target.Path, "B", "file"), now, now); err != nil {
		t.Fatal(err)
	}
	p, err := ScanTarget(context.Background(), target, ScanOptions{OlderThan: "30d"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Entries) != 2 || p.Entries[0].Name != "A" || p.Entries[1].Name != "C" {
		t.Fatalf("entries=%+v", p.Entries)
	}
}

func TestBoundedScanConcurrency(t *testing.T) {
	target := fixtureTarget(t)
	var active, peak atomic.Int32
	_, err := scanTarget(context.Background(), target, ScanOptions{Workers: 100}, nil, func(ctx context.Context, path string, activity bool) DerivedEntry {
		n := active.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		defer active.Add(-1)
		return scanEntry(ctx, path, activity)
	})
	if err != nil {
		t.Fatal(err)
	}
	if peak.Load() > 3 {
		t.Fatal(fmt.Sprintf("too many workers: %d", peak.Load()))
	}
}

func TestModifiedRegularEntryIsSkipped(t *testing.T) {
	target := Target{Name: "test", Path: t.TempDir()}
	path := filepath.Join(target.Path, "cache")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("updated"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := DeletePlan(context.Background(), plan, nil)
	if err == nil || result.Items != 0 || result.Bytes != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("changed file removed", err)
	}
}
