package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setupTrashArchives(t *testing.T) (string, string) {
	t.Helper()
	original := ArchivesTarget
	t.Cleanup(func() { ArchivesTarget = original })
	home := t.TempDir()
	t.Setenv("HOME", home)
	archives := filepath.Join(home, "Archives")
	entry := filepath.Join(archives, "example.xcarchive")
	if err := os.MkdirAll(entry, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "data"), []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	ArchivesTarget.Path = archives
	return entry, filepath.Join(home, ".Trash")
}

func TestArchivesTrashDryRunNeverMovesOrReads(t *testing.T) {
	entry, trash := setupTrashArchives(t)
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(forbiddenInput{})
	root.SetArgs([]string{"clean", "archives", "--trash", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(trash); !os.IsNotExist(err) {
		t.Fatal("dry run created Trash", err)
	}
	for _, text := range []string{"example.xcarchive", "Disk space is reclaimed only after emptying Trash"} {
		if !strings.Contains(out.String(), text) {
			t.Fatal(out.String())
		}
	}
	if strings.Contains(out.String(), "Nuked") || strings.Contains(out.String(), "Freed") {
		t.Fatal(out.String())
	}
}

func TestArchivesTrashConfirmationAndMove(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Trash")
	}
	entry, trash := setupTrashArchives(t)
	out, err := executeCommandWithInput(NewRootCommand(), "n\n", "clean", "archives", "--trash")
	if err != nil || !strings.Contains(out, "Move to Trash Xcode Archives") {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(trash); !os.IsNotExist(err) {
		t.Fatal("declined operation created Trash", err)
	}
	out, err = executeCommand(NewRootCommand(), "clean", "archives", "--trash", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Nuked") || strings.Contains(out, "Freed") || !strings.Contains(out, "Moved estimated 3 B") {
		t.Fatal(out)
	}
	data, err := os.ReadFile(filepath.Join(trash, "example.xcarchive", "data"))
	if err != nil || string(data) != "abc" {
		t.Fatal(string(data), err)
	}
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Fatal("archive not moved", err)
	}
	if _, err := os.Stat(ArchivesTarget.Path); err != nil {
		t.Fatal(err)
	}
}

func TestTrashFlagOnlyOnArchives(t *testing.T) {
	for _, target := range []string{"derived", "spm", "caches", "simulators", "module-cache", "device-support"} {
		if _, err := executeCommand(NewRootCommand(), "clean", target, "--trash", "--dry-run"); err == nil {
			t.Fatal(target)
		}
	}
	out, err := executeCommand(NewRootCommand(), "clean", "archives", "--help")
	if err != nil || !strings.Contains(out, "--trash") || !strings.Contains(out, "emptying Trash") {
		t.Fatal(out, err)
	}
}
