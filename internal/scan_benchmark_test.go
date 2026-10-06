package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Fixtures model DerivedData's build/intermediate/index trees on the current
// filesystem. Setup is outside all timed sub-benchmarks; no user caches are touched.
func BenchmarkScan(b *testing.B) {
	cases := []struct {
		name                   string
		projects, depth, files int
		filter                 string
	}{
		{"DeepTrees", 4, 40, 8, ""},
		{"SmallFiles", 8, 1, 1000, ""},
		{"ManyProjects", 200, 1, 10, ""},
		{"ProjectFilter", 100, 2, 20, "^Project0000$"},
		{"MacOSBuildCache", 12, 6, 100, ""},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			dir := b.TempDir()
			data := make([]byte, 1024)
			for p := 0; p < tc.projects; p++ {
				path := filepath.Join(dir, fmt.Sprintf("Project%04d", p))
				for d := 0; d < tc.depth; d++ {
					path = filepath.Join(path, fmt.Sprintf("Build-%02d.noindex", d))
					if err := os.MkdirAll(path, 0755); err != nil {
						b.Fatal(err)
					}
					for f := 0; f < tc.files; f++ {
						if err := os.WriteFile(filepath.Join(path, fmt.Sprintf("file-%04d.o", f)), data, 0644); err != nil {
							b.Fatal(err)
						}
					}
				}
			}
			target := Target{Name: tc.name, Path: dir}
			for _, legacy := range []struct {
				name   string
				passes int
			}{{"LegacyScan", 1}, {"LegacyCleanup", 2}, {"LegacyAll", 3}} {
				b.Run(legacy.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						for range legacy.passes {
							if err := legacyScan(dir); err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
			for _, workers := range []int{1, 4} {
				b.Run(fmt.Sprintf("PlanWorkers%d", workers), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						if _, err := ScanTarget(context.Background(), target, ScanOptions{Project: tc.filter, Activity: true, Workers: workers}, nil); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

// Mirrors the previous metadata traversal: top-level Info followed by Walk,
// which stats every directory and file. Legacy filtered commands scanned all
// projects before applying their regex.
func legacyScan(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var size int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			continue
		}
		err = filepath.Walk(filepath.Join(path, entry.Name()), func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				size += info.Size()
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	benchmarkBytes = size
	return nil
}

var benchmarkBytes int64
