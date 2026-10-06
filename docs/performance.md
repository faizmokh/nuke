# Scan performance

Measured on October 1, 2026, on macOS arm64, Apple M3 Pro, Go 1.27.1.
These are warm-cache timings on generated temporary filesystem fixtures, not
measurements of a user's live DerivedData or total deletion time.

## Reproduce

```bash
go test ./internal -run '^$' -bench '^BenchmarkScan$' -benchmem -benchtime=3x -count=2
```

Fixture creation is outside timed sections. Each row below gives the range from
two benchmark runs, each with three iterations. The suite covers deep directory
trees, many small files, many projects, selective regexes, and a nested build-cache
layout. The build-cache fixture has 12 projects with 6 levels and 100 files per
level (7,200 files, 1 KiB each).

The legacy baseline mirrors the old top-level enumeration and `filepath.Walk`
metadata traversal. `LegacyCleanup` repeats that walk twice to represent summary
scanning plus the old deletion-time size calculation. `LegacyAll` repeats it three
times to include the former post-confirmation rescan. These baselines omit actual
removal and confirmation, so they isolate avoidable metadata work.

## Results

| Fixture | Legacy single scan | New, 1 worker | New, 4 workers |
|---------|--------------------|---------------|----------------|
| Deep trees | 10.13–10.34 ms | 8.58–8.59 ms | 2.97–3.00 ms |
| Many small files | 22.32–22.70 ms | 19.12–19.63 ms | 6.62–6.84 ms |
| Many projects | 13.64–14.38 ms | 13.87–13.88 ms | 4.35–4.50 ms |
| Select 1 of 100 projects | 17.76–18.04 ms | 0.24–0.29 ms | 0.21–0.22 ms |
| macOS build-cache layout | 23.03–23.67 ms | 19.78–20.39 ms | 6.86–7.03 ms |

The selective case launches only one worker because only one project qualifies;
its improvement comes from avoiding unrelated traversal. The legacy baseline
scans all projects, matching the previous filter-after-scan behavior.

For the build-cache fixture, the former two-pass metadata work took
45.59–46.02 ms, and three-pass work took 69.13–70.27 ms. The new plan is scanned
once and reused for deletion. This does not imply equivalent end-to-end cleanup
speedups: actual removal still depends on file count and filesystem state.

## Allocations and tradeoffs

On the build-cache fixture, a legacy single walk allocated about 4.76 MB and
30,287 objects, versus 6.61 MB and about 44,702 objects for the new four-worker
plan. WalkDir entries, retained identities, and the plan increase single-pass
allocations. Replacing the old two or three traversals reduces metadata-phase
allocation totals from approximately 9.52 MB or 14.27 MB to 6.61 MB.

Four workers improved elapsed scanning time across these local fixtures, so the
bounded default remains four, capped by CPU and eligible entry count. Results
may differ on cold caches or other storage. No persistent cache or parallel
removal is used. Progress is serialized and UI rendering is capped at 10 Hz.
