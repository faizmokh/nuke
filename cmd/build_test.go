package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOnlyCurrentUsesCustomRoot(t *testing.T) {
	cache, _ := currentFixture(t)
	for _, name := range []string{"App-match", "App-unrelated"} {
		statusFile(t, filepath.Join(cache, name, "Build", "Products", "app"), "123")
		statusFile(t, filepath.Join(cache, name, "Build", "Intermediates.noindex", "object"), "12345")
		statusFile(t, filepath.Join(cache, name, "Index.noindex", "index"), "index")
		statusFile(t, filepath.Join(cache, name, "SourcePackages", "checkouts", "file"), "checkout")
	}
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetIn(forbiddenInput{})
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"clean", "derived", "--current", "--build-only", "--derived-data", cache, "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "8 B") || strings.Contains(out.String(), "App-unrelated") {
		t.Fatal(out.String())
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match", "Build", "Products")); err != nil {
		t.Fatal("preview deleted outputs", err)
	}
	if _, err := executeCommand(NewRootCommand(), "clean", "derived", "--current", "--build-only", "--derived-data", cache, "--yes"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"App-match/Index.noindex/index", "App-match/SourcePackages/checkouts/file", "App-match/info.plist", "App-unrelated/Build/Products/app", "ModuleCache.noindex/file"} {
		if _, err := os.Stat(filepath.Join(cache, path)); err != nil {
			t.Fatal("preserved path removed", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match", "Build", "Products")); !os.IsNotExist(err) {
		t.Fatal("matching outputs remain", err)
	}
}
func TestBuildOnlySelectionAndConfirmation(t *testing.T) {
	cache, _ := currentFixture(t)
	statusFile(t, filepath.Join(cache, "App-match", "Build", "Products", "app"), "product")
	text, err := executeCommandWithInput(NewRootCommand(), "1\ny\n", "clean", "derived", "--build-only", "--derived-data", cache)
	if err != nil {
		t.Fatal(text, err)
	}
	if !strings.Contains(text, "DerivedData build outputs") {
		t.Fatal("confirmation scope unclear", text)
	}
	if _, err := os.Stat(filepath.Join(cache, "App-match", "Build", "Products")); !os.IsNotExist(err) {
		t.Fatal("selection not deleted", err)
	}
}
