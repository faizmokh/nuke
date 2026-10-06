package internal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Target struct{ Name, Path string }
type Result struct {
	Target  Target
	Bytes   int64
	Items   int
	Deleted bool
}

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func Scan(target Target) (int64, int, error) {
	p, err := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	size, count := p.Summary()
	return size, count, err
}

func Confirm(w io.Writer, r io.Reader, target Target, bytes int64, items int) bool {
	fmt.Fprintf(w, "%s: %s in %d items\nNuke %s? [y/N] ", target.Name, HumanSize(bytes), items, HumanSize(bytes))
	ok, _ := ReadConfirmation(bufio.NewReader(r))
	return ok
}

func ReadConfirmation(r *bufio.Reader) (bool, error) {
	response, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

func Nuke(target Target, onProgress func(int, int)) (int64, error) {
	p, scanErr := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if p.rootInfo == nil {
		return 0, scanErr
	}
	result, err := DeletePlan(context.Background(), p, onProgress)
	return result.Bytes, errors.Join(scanErr, err)
}

func Run(w io.Writer, r io.Reader, target Target, yes, dryRun bool) error {
	p, err := ScanTarget(context.Background(), target, ScanOptions{}, nil)
	if err != nil {
		return err
	}
	size, count := p.Summary()
	if count == 0 {
		fmt.Fprintf(w, "%s: nothing to clean\n", target.Name)
		return nil
	}
	fmt.Fprintf(w, "%s: %s in %d items\n", target.Name, HumanSize(size), count)
	if dryRun {
		return nil
	}
	if !yes {
		fmt.Fprintf(w, "Nuke %s? [y/N] ", HumanSize(size))
		ok, err := ReadConfirmation(bufio.NewReader(r))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	result, err := DeletePlan(context.Background(), p, nil)
	fmt.Fprintf(w, "Nuked estimated %s from %s\n", HumanSize(result.Bytes), target.Name)
	return err
}
