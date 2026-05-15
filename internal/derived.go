package internal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DerivedEntry struct {
	Name         string
	Path         string
	Size         int64
	LastActivity time.Time
}

func ScanDerived(target Target) ([]DerivedEntry, error) {
	path := ExpandHome(target.Path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", target.Name, err)
	}

	derived := make([]DerivedEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := DerivedEntry{
			Name: entry.Name(),
			Path: entryPath,
		}

		if entry.IsDir() {
			size, latest, err := dirStats(entryPath)
			if err != nil {
				continue
			}
			item.Size = size
			item.LastActivity = latest
		} else {
			item.Size = info.Size()
			item.LastActivity = info.ModTime()
		}

		derived = append(derived, item)
	}

	sort.Slice(derived, func(i, j int) bool {
		return derived[i].Name < derived[j].Name
	})

	return derived, nil
}

func dirStats(path string) (int64, time.Time, error) {
	var size int64
	var latest time.Time
	var sawFile bool
	rootInfo, err := os.Stat(path)
	if err != nil {
		return 0, time.Time{}, err
	}
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			sawFile = true
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			size += info.Size()
		}
		return nil
	})
	if !sawFile {
		latest = rootInfo.ModTime()
	}
	return size, latest, err
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
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid age threshold %q", s)
	}

	now := time.Now()
	switch s[len(s)-1] {
	case 'd':
		return now.Add(-time.Duration(amount) * 24 * time.Hour), nil
	case 'w':
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
		fmt.Fprintf(w, "%2d  %-24s %-9s %s\n", i+1, entry.Name, HumanSize(entry.Size), entry.LastActivity.Format("2006-01-02"))
	}
}

func InteractiveSelect(w io.Writer, r io.Reader, entries []DerivedEntry) ([]DerivedEntry, error) {
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
	var freed int64
	for i, entry := range entries {
		freed += entry.Size
		if err := os.RemoveAll(entry.Path); err != nil {
			return freed, err
		}
		if onProgress != nil {
			onProgress(i+1, len(entries))
		}
	}
	return freed, nil
}

func EntriesSummary(entries []DerivedEntry) (int64, int) {
	var total int64
	for _, entry := range entries {
		total += entry.Size
	}
	return total, len(entries)
}
