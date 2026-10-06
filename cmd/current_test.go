package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func currentFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "App.xcodeproj")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	cache := filepath.Join(t.TempDir(), "custom-cache")
	other := filepath.Join(t.TempDir(), "App.xcodeproj")
	if err := os.Mkdir(other, 0755); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"App-match": project, "App-unrelated": other} {
		statusFile(t, filepath.Join(cache, name, "info.plist"), fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict><key>WorkspacePath</key><string>%s</string></dict></plist>`, path))
		statusFile(t, filepath.Join(cache, name, "file"), "data")
	}
	statusFile(t, filepath.Join(cache, "ModuleCache.noindex", "file"), "module")
	return cache, project
}
func TestCurrentOnlyDeletesMatchingProject(t *testing.T) {
	cache, _ := currentFixture(t)
	text, err := executeCommand(NewRootCommand(), "clean", "derived", "--current", "--derived-data", cache, "--yes")
	if err != nil {
		t.Fatal(text, err)
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match")); !os.IsNotExist(err) {
		t.Fatal("matching entry remains", err)
	}
	for _, name := range []string{"App-unrelated", "ModuleCache.noindex"} {
		if _, err := os.Stat(filepath.Join(cache, name)); err != nil {
			t.Fatal("unrelated entry removed", err)
		}
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatal("custom root removed", err)
	}
}
func TestCurrentPreviewDoesNotReadInput(t *testing.T) {
	cache, _ := currentFixture(t)
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(forbiddenInput{})
	root.SetArgs([]string{"--derived-data", cache, "clean", "derived", "--current", "--list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "App-match") || strings.Contains(out.String(), "App-unrelated") {
		t.Fatal(out.String())
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match")); err != nil {
		t.Fatal(err)
	}
}
func TestCurrentConflictingFlags(t *testing.T) {
	for _, args := range [][]string{{"clean", "derived", "--current", "--all", "--yes"}, {"clean", "derived", "--current", "--project", "App"}} {
		if _, err := executeCommand(NewRootCommand(), args...); err == nil {
			t.Fatal("conflicting flags accepted")
		}
	}
}
func TestCustomRootUsedForStatusAndModuleCache(t *testing.T) {
	setupStatus(t)
	cache, _ := currentFixture(t)
	text, err := executeCommand(NewRootCommand(), "status", "--derived-data", cache)
	if err != nil || !strings.Contains(text, "included in DerivedData") {
		t.Fatalf("text=%s err=%v", text, err)
	}
	if _, err := executeCommand(NewRootCommand(), "clean", "module-cache", "--derived-data", cache, "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "ModuleCache.noindex")); err != nil {
		t.Fatal("module root must remain", err)
	}
	entries, err := os.ReadDir(filepath.Join(cache, "ModuleCache.noindex"))
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match")); err != nil {
		t.Fatal("project removed by module cleanup", err)
	}
}

func TestCurrentMalformedMetadataNeverDeletes(t *testing.T) {
	cache, _ := currentFixture(t)
	statusFile(t, filepath.Join(cache, "Z-broken", "info.plist"), "not a plist")
	text, err := executeCommand(NewRootCommand(), "clean", "derived", "--current", "--derived-data", cache, "--yes")
	if err == nil {
		t.Fatal("malformed metadata ignored", text)
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match")); err != nil {
		t.Fatal("matching entry deleted before filter failed", err)
	}
}

func TestCustomRootUsedByAll(t *testing.T) {
	setupStatus(t)
	cache, _ := currentFixture(t)
	statusFile(t, filepath.Join(SPMTarget.Path, "file"), "cache")
	if _, err := executeCommand(NewRootCommand(), "clean", "caches", "--derived-data", cache, "--yes"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatal("custom root not cleaned", entries, err)
	}
}
