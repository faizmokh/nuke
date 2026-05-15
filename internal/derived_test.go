package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanDerived(t *testing.T) {
	dir := t.TempDir()
	target := Target{Name: "derived", Path: dir}

	projectA := filepath.Join(dir, "MyApp-abc123")
	projectB := filepath.Join(dir, "OtherApp-def456")
	if err := os.MkdirAll(projectA, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(projectB, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	older := time.Date(2025, time.January, 10, 12, 0, 0, 0, time.UTC)
	newer := older.Add(48 * time.Hour)
	writeTestFileWithTime(t, filepath.Join(projectA, "old.txt"), "hello", older)
	writeTestFileWithTime(t, filepath.Join(projectA, "new.txt"), "world!", newer)
	writeTestFileWithTime(t, filepath.Join(projectB, "main.txt"), "swift", older)

	entries, err := ScanDerived(target)
	if err != nil {
		t.Fatalf("ScanDerived() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	entryA := findDerivedEntry(t, entries, "MyApp-abc123")
	if entryA.Size != int64(len("hello")+len("world!")) {
		t.Errorf("entryA.Size = %d, want %d", entryA.Size, len("hello")+len("world!"))
	}
	if !entryA.LastActivity.Equal(newer) {
		t.Errorf("entryA.LastActivity = %v, want %v", entryA.LastActivity, newer)
	}

	entryB := findDerivedEntry(t, entries, "OtherApp-def456")
	if entryB.Size != int64(len("swift")) {
		t.Errorf("entryB.Size = %d, want %d", entryB.Size, len("swift"))
	}
	if !entryB.LastActivity.Equal(older) {
		t.Errorf("entryB.LastActivity = %v, want %v", entryB.LastActivity, older)
	}
}

func TestScanDerivedEmpty(t *testing.T) {
	dir := t.TempDir()
	entries, err := ScanDerived(Target{Name: "derived", Path: dir})
	if err != nil {
		t.Fatalf("ScanDerived() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

func TestFilterByAge(t *testing.T) {
	cutoff := time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC)
	entries := []DerivedEntry{
		{Name: "old", LastActivity: cutoff.Add(-24 * time.Hour)},
		{Name: "exact", LastActivity: cutoff},
		{Name: "new", LastActivity: cutoff.Add(24 * time.Hour)},
	}

	filtered := FilterByAge(entries, cutoff)
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].Name != "old" {
		t.Errorf("filtered[0].Name = %q, want %q", filtered[0].Name, "old")
	}
}

func TestFilterByProject(t *testing.T) {
	entries := []DerivedEntry{{Name: "MyApp-abc123"}, {Name: "OtherApp-def456"}}

	filtered, err := FilterByProject(entries, "My.*")
	if err != nil {
		t.Fatalf("FilterByProject() error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].Name != "MyApp-abc123" {
		t.Errorf("filtered[0].Name = %q, want %q", filtered[0].Name, "MyApp-abc123")
	}
}

func TestFilterByProjectInvalidRegex(t *testing.T) {
	_, err := FilterByProject([]DerivedEntry{{Name: "MyApp-abc123"}}, "[")
	if err == nil {
		t.Fatal("FilterByProject() error = nil, want invalid regex error")
	}
}

func TestParseAgeThreshold(t *testing.T) {
	before := time.Now()
	relative, err := ParseAgeThreshold("2w")
	after := time.Now()
	if err != nil {
		t.Fatalf("ParseAgeThreshold(2w) error: %v", err)
	}
	if relative.Before(before.Add(-14*24*time.Hour)) || relative.After(after.Add(-14*24*time.Hour+2*time.Second)) {
		t.Errorf("relative threshold = %v, want about 2 weeks ago", relative)
	}

	absolute, err := ParseAgeThreshold("2025-01-01")
	if err != nil {
		t.Fatalf("ParseAgeThreshold(2025-01-01) error: %v", err)
	}
	want := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !absolute.Equal(want) {
		t.Errorf("absolute threshold = %v, want %v", absolute, want)
	}

	if _, err := ParseAgeThreshold("later"); err == nil {
		t.Fatal("ParseAgeThreshold(later) error = nil, want invalid threshold error")
	}
}

func TestInteractiveSelect(t *testing.T) {
	entries := []DerivedEntry{
		{Name: "MyApp-abc123", Size: 10, LastActivity: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "OtherApp-def456", Size: 20, LastActivity: time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)},
		{Name: "ThirdApp-ghi789", Size: 30, LastActivity: time.Date(2025, time.January, 3, 0, 0, 0, 0, time.UTC)},
	}

	var out bytes.Buffer
	selected, err := InteractiveSelect(&out, strings.NewReader("1, 3\n"), entries)
	if err != nil {
		t.Fatalf("InteractiveSelect() error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	if selected[0].Name != "MyApp-abc123" || selected[1].Name != "ThirdApp-ghi789" {
		t.Fatalf("selected = %#v, want first and third entries", selected)
	}
	if !strings.Contains(out.String(), "Delete which?") {
		t.Errorf("InteractiveSelect() output = %q, want prompt", out.String())
	}
}

func TestInteractiveSelectAllAndNone(t *testing.T) {
	entries := []DerivedEntry{{Name: "MyApp-abc123"}, {Name: "OtherApp-def456"}}

	selectedAll, err := InteractiveSelect(&bytes.Buffer{}, strings.NewReader("a\n"), entries)
	if err != nil {
		t.Fatalf("InteractiveSelect(all) error: %v", err)
	}
	if len(selectedAll) != 2 {
		t.Fatalf("len(selectedAll) = %d, want 2", len(selectedAll))
	}

	selectedNone, err := InteractiveSelect(&bytes.Buffer{}, strings.NewReader("n\n"), entries)
	if err != nil {
		t.Fatalf("InteractiveSelect(none) error: %v", err)
	}
	if len(selectedNone) != 0 {
		t.Fatalf("len(selectedNone) = %d, want 0", len(selectedNone))
	}
}

func TestNukeEntries(t *testing.T) {
	dir := t.TempDir()
	projectA := filepath.Join(dir, "MyApp-abc123")
	projectB := filepath.Join(dir, "OtherApp-def456")
	if err := os.MkdirAll(projectA, 0755); err != nil {
		t.Fatalf("MkdirAll(projectA) error: %v", err)
	}
	if err := os.MkdirAll(projectB, 0755); err != nil {
		t.Fatalf("MkdirAll(projectB) error: %v", err)
	}
	writeTestFileWithTime(t, filepath.Join(projectA, "main.txt"), "hello", time.Now())
	writeTestFileWithTime(t, filepath.Join(projectB, "main.txt"), "world", time.Now())

	entries, err := ScanDerived(Target{Name: "derived", Path: dir})
	if err != nil {
		t.Fatalf("ScanDerived() error: %v", err)
	}
	entryA := findDerivedEntry(t, entries, "MyApp-abc123")

	freed, err := NukeEntries([]DerivedEntry{entryA}, nil)
	if err != nil {
		t.Fatalf("NukeEntries() error: %v", err)
	}
	if freed == 0 {
		t.Fatal("NukeEntries() freed = 0, want > 0")
	}
	if _, err := os.Stat(projectA); !os.IsNotExist(err) {
		t.Fatalf("projectA still exists, stat err = %v", err)
	}
	if _, err := os.Stat(projectB); err != nil {
		t.Fatalf("projectB missing after partial delete: %v", err)
	}
}

func writeTestFileWithTime(t *testing.T, path string, contents string, modTime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) error: %v", path, err)
	}
}

func findDerivedEntry(t *testing.T, entries []DerivedEntry, name string) DerivedEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("entry %q not found", name)
	return DerivedEntry{}
}
