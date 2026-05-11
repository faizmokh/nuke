package internal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Target struct {
	Name string
	Path string
}

type Result struct {
	Target  Target
	Bytes   int64
	Items   int
	Deleted bool
}

func ExpandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func Scan(target Target) (int64, int, error) {
	path := ExpandHome(target.Path)
	var total int64
	var items int

	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, fmt.Errorf("scanning %s: %w", target.Name, err)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.IsDir() {
			size, err := dirSize(filepath.Join(path, entry.Name()))
			if err != nil {
				continue
			}
			total += size
		} else {
			total += info.Size()
		}
		items++
	}

	return total, items, nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func Confirm(w io.Writer, r io.Reader, target Target, bytes int64, items int) bool {
	fmt.Fprintf(w, "%s: %s in %d items\n", target.Name, HumanSize(bytes), items)
	fmt.Fprintf(w, "Nuke %s? [y/N] ", HumanSize(bytes))

	reader := bufio.NewReader(r)
	response, _ := reader.ReadString('\n')
	response = trimNewline(response)

	return response == "y" || response == "Y" || response == "yes"
}

func Nuke(target Target, onProgress func(current, total int)) (int64, error) {
	path := ExpandHome(target.Path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", target.Name, err)
	}

	var freed int64
	for i, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			s, _ := dirSize(entryPath)
			freed += s
			os.RemoveAll(entryPath)
		} else {
			info, _ := entry.Info()
			freed += info.Size()
			os.Remove(entryPath)
		}
		if onProgress != nil {
			onProgress(i+1, len(entries))
		}
	}

	return freed, nil
}

func Run(w io.Writer, r io.Reader, target Target, yes bool, dryRun bool) error {
	bytes, items, err := Scan(target)
	if err != nil {
		return err
	}

	if items == 0 {
		fmt.Fprintf(w, "%s: nothing to clean\n", target.Name)
		return nil
	}

	fmt.Fprintf(w, "%s: %s in %d items\n", target.Name, HumanSize(bytes), items)

	if dryRun {
		return nil
	}

	if !yes {
		fmt.Fprintf(w, "Nuke %s? [y/N] ", HumanSize(bytes))
		reader := bufio.NewReader(r)
		response, _ := reader.ReadString('\n')
		response = trimNewline(response)
		if response != "y" && response != "Y" && response != "yes" {
			return nil
		}
	}

	bar := NewProgressBar(w, items)
	freed, err := Nuke(target, func(current, total int) {
		bar.Update(current)
	})
	bar.Done()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Nuked %s from %s\n", HumanSize(freed), target.Name)
	return nil
}

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	return s
}
