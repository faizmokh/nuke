package internal

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"time"
)

// ScanPlan is a per-invocation snapshot. Deletion consumes these entries without
// measuring them again. Root and entry identities are private to prevent forged plans.
type ScanPlan struct {
	Target   Target
	Entries  []DerivedEntry
	rootPath string
	rootInfo os.FileInfo
}

type ScanOptions struct {
	Project    string
	OlderThan  string
	Workers    int // Internal tuning for benchmarks; zero chooses the bounded default.
	Activity   bool
	BuildOnly  bool
	MatchEntry func(context.Context, string) (bool, error) // Optional metadata filter, before recursive scans.
}

type ScanUpdate struct {
	Entry       DerivedEntry
	Done, Total int
	Complete    bool
}

func (p ScanPlan) Summary() (int64, int) {
	var size int64
	var count int
	for _, e := range p.Entries {
		if e.Err == nil {
			size += e.Size
			count++
		}
	}
	return size, count
}

func (p ScanPlan) Err() error {
	var errs []error
	for _, e := range p.Entries {
		if e.Err != nil {
			errs = append(errs, fmt.Errorf("scanning %s: %w", e.Name, e.Err))
		}
	}
	return errors.Join(errs...)
}

func (p ScanPlan) Select(entries []DerivedEntry) ScanPlan {
	selected := make(map[string]bool, len(entries))
	for _, e := range entries {
		selected[e.Path] = true
	}
	result := p
	result.Entries = nil
	for _, e := range p.Entries {
		if selected[e.Path] && e.Err == nil {
			result.Entries = append(result.Entries, e)
		}
	}
	return result
}

func ScanTarget(ctx context.Context, target Target, options ScanOptions, onUpdate func(ScanUpdate)) (ScanPlan, error) {
	return scanTarget(ctx, target, options, onUpdate, scanEntry)
}

// The stats function is injectable internally so tests can assert traversal counts
// and failure handling without relying on filesystem permissions or timing.
func scanTarget(ctx context.Context, target Target, options ScanOptions, onUpdate func(ScanUpdate), stats func(context.Context, string, bool) DerivedEntry) (ScanPlan, error) {
	plan := ScanPlan{Target: target}
	var matcher *regexp.Regexp
	var threshold *time.Time
	if options.Project != "" {
		var err error
		matcher, err = regexp.Compile(options.Project)
		if err != nil {
			return plan, fmt.Errorf("invalid project pattern: %w", err)
		}
	}
	if options.OlderThan != "" {
		t, err := ParseAgeThreshold(options.OlderThan)
		if err != nil {
			return plan, err
		}
		threshold = &t
		options.Activity = true
	}
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	path, err := filepath.EvalSymlinks(ExpandHome(target.Path))
	if err != nil {
		return plan, fmt.Errorf("scanning %s: %w", target.Name, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return plan, err
	}
	plan.rootPath = path
	plan.rootInfo, err = os.Stat(path)
	if err != nil {
		return plan, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return plan, fmt.Errorf("scanning %s: %w", target.Name, err)
	}
	for _, e := range entries {
		if matcher != nil && !matcher.MatchString(e.Name()) {
			continue
		}
		entryPath := filepath.Join(path, e.Name())
		if options.BuildOnly {
			eligible, err := hasBuildOutputs(entryPath)
			if err != nil {
				plan.Entries = nil
				return plan, fmt.Errorf("checking build outputs for %s: %w", e.Name(), err)
			}
			if !eligible {
				continue
			}
		}
		if options.MatchEntry != nil {
			match, err := options.MatchEntry(ctx, entryPath)
			if err != nil {
				plan.Entries = nil
				return plan, fmt.Errorf("matching %s: %w", e.Name(), err)
			}
			if !match {
				continue
			}
		}
		plan.Entries = append(plan.Entries, DerivedEntry{Name: e.Name(), Path: filepath.Join(path, e.Name())})
	}
	total := len(plan.Entries)
	if total == 0 {
		return plan, nil
	}
	if onUpdate != nil && threshold == nil {
		for _, e := range plan.Entries {
			onUpdate(ScanUpdate{Entry: e, Total: total})
		}
	}
	if options.BuildOnly {
		stats = scanBuildEntry
	}
	workers := options.Workers
	if workers <= 0 {
		workers = 4
	}
	workers = min(workers, 4, runtime.NumCPU(), total)
	workers = max(workers, 1)
	type result struct {
		index int
		entry DerivedEntry
	}
	jobs := make(chan int)
	results := make(chan result, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				e := stats(ctx, plan.Entries[i].Path, options.Activity)
				select {
				case results <- result{i, e}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs)
		for i := range plan.Entries {
			if ctx.Err() != nil {
				return
			}
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()
	done := 0
	// Only this goroutine mutates the plan or calls the observer.
	for r := range results {
		plan.Entries[r.index] = r.entry
		done++
		if onUpdate != nil && (threshold == nil || r.entry.Err != nil || r.entry.LastActivity.Before(*threshold)) {
			onUpdate(ScanUpdate{Entry: r.entry, Done: done, Total: total, Complete: true})
		}
	}
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	if threshold != nil {
		eligible := plan.Entries[:0]
		for _, e := range plan.Entries {
			if e.Err != nil || e.LastActivity.Before(*threshold) {
				eligible = append(eligible, e)
			}
		}
		plan.Entries = eligible
	}
	sort.Slice(plan.Entries, func(i, j int) bool { return plan.Entries[i].Name < plan.Entries[j].Name })
	return plan, plan.Err()
}

func scanEntry(ctx context.Context, path string, activity bool) DerivedEntry {
	e := DerivedEntry{Name: filepath.Base(path), Path: path}
	info, err := os.Lstat(path)
	if err != nil {
		e.Err = err
		return e
	}
	e.identity = info
	e.LastActivity = info.ModTime()
	if !info.IsDir() {
		e.Size = info.Size()
		return e
	}
	var latest time.Time
	sawFile := false
	err = filepath.WalkDir(path, func(current string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		e.Size += fi.Size()
		if activity {
			sawFile = true
			if fi.ModTime().After(latest) {
				latest = fi.ModTime()
			}
		}
		return nil
	})
	if activity && sawFile {
		e.LastActivity = latest
	}
	// Reject entries changed while scanning; don't offer a partially measured entry.
	if err == nil {
		after, statErr := os.Lstat(path)
		if statErr != nil {
			err = statErr
		} else if !sameEntry(info, after) {
			err = errors.New("entry changed during scan")
		}
	}
	e.Err = err
	return e
}

func sameEntry(before, after os.FileInfo) bool {
	return before != nil && after != nil && os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}
