package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DerivedEntry struct {
	Name          string
	Path          string
	Size          int64
	LastActivity  time.Time
	Err           error
	identity      os.FileInfo
	buildOnly     bool
	buildIdentity os.FileInfo
	buildParts    []buildPart
}

type DerivedScanUpdate struct {
	Index    int
	Entry    DerivedEntry
	Done     int
	Total    int
	Complete bool
}

func ScanDerived(target Target) ([]DerivedEntry, error) {
	plan, err := ScanTarget(context.Background(), target, ScanOptions{Activity: true, Workers: 1}, nil)
	return plan.Entries, err
}

// Compatibility adapter. New callers use ScanTarget and path-keyed ScanUpdate.
func ScanDerivedProgressively(target Target, onUpdate func(DerivedScanUpdate)) error {
	indexes := map[string]int{}
	_, err := ScanTarget(context.Background(), target, ScanOptions{Activity: true, Workers: 1}, func(u ScanUpdate) {
		index, ok := indexes[u.Entry.Path]
		if !ok {
			index = len(indexes)
			indexes[u.Entry.Path] = index
		}
		if onUpdate != nil {
			onUpdate(DerivedScanUpdate{Index: index, Entry: u.Entry, Done: u.Done, Total: u.Total, Complete: u.Complete})
		}
	})
	return err
}

func FilterByAge(entries []DerivedEntry, threshold time.Time) []DerivedEntry {
	filtered := make([]DerivedEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.LastActivity.Before(threshold) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func FilterByProject(entries []DerivedEntry, pattern string) ([]DerivedEntry, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid project pattern: %w", err)
	}

	filtered := make([]DerivedEntry, 0, len(entries))
	for _, entry := range entries {
		if re.MatchString(entry.Name) {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

func ParseAgeThreshold(s string) (time.Time, error) {
	if threshold, err := time.Parse("2006-01-02", s); err == nil {
		return threshold, nil
	}

	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid age threshold %q", s)
	}

	amount, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || amount <= 0 || amount > 100000 {
		return time.Time{}, fmt.Errorf("invalid age threshold %q", s)
	}

	now := time.Now()
	switch s[len(s)-1] {
	case 'd':
		return now.Add(-time.Duration(amount) * 24 * time.Hour), nil
	case 'w':
		if amount > int((1<<63-1)/(7*24*time.Hour)) {
			return time.Time{}, fmt.Errorf("invalid age threshold %q", s)
		}
		return now.Add(-time.Duration(amount) * 7 * 24 * time.Hour), nil
	case 'm':
		return now.AddDate(0, -amount, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid age threshold %q", s)
	}
}

func FormatEntriesTable(w io.Writer, entries []DerivedEntry) {
	fmt.Fprintln(w, "#  Project                  Size      Last Activity")
	for i, entry := range entries {
		if entry.Err != nil {
			fmt.Fprintf(w, "%2d  %s  unavailable: %v\n", i+1, entry.Name, entry.Err)
			continue
		}
		fmt.Fprintf(w, "%2d  %-24s %-9s %s\n", i+1, entry.Name, HumanSize(entry.Size), entry.LastActivity.Format("2006-01-02"))
	}
}

func InteractiveSelect(w io.Writer, r io.Reader, entries []DerivedEntry) ([]DerivedEntry, error) {
	valid := make([]DerivedEntry, 0, len(entries))
	for _, e := range entries {
		if e.Err == nil {
			valid = append(valid, e)
		}
	}
	entries = valid
	FormatEntriesTable(w, entries)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "a  Select all")
	fmt.Fprintln(w, "n  Select none")
	fmt.Fprint(w, "Delete which? [1,2,3,a,n]: ")

	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	response = strings.TrimSpace(response)

	switch strings.ToLower(response) {
	case "a":
		selected := make([]DerivedEntry, len(entries))
		copy(selected, entries)
		return selected, nil
	case "n", "":
		return nil, nil
	}

	parts := strings.Split(response, ",")
	selected := make([]DerivedEntry, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		index, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || index < 1 || index > len(entries) {
			return nil, fmt.Errorf("invalid selection %q", strings.TrimSpace(part))
		}
		index--
		if _, ok := seen[index]; ok {
			continue
		}
		seen[index] = struct{}{}
		selected = append(selected, entries[index])
	}

	return selected, nil
}

func NukeEntries(entries []DerivedEntry, onProgress func(current, total int)) (int64, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	root := filepath.Dir(entries[0].Path)
	info, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	result, err := DeletePlan(context.Background(), ScanPlan{Target: Target{Path: root}, Entries: entries, rootPath: root, rootInfo: info}, onProgress)
	return result.Bytes, err
}

func EntriesSummary(entries []DerivedEntry) (int64, int) {
	var total int64
	for _, entry := range entries {
		total += entry.Size
	}
	return total, len(entries)
}
