package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func projectDir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
func TestFindCurrentProject(t *testing.T) {
	root := t.TempDir()
	project := projectDir(t, filepath.Join(root, "App.xcodeproj"))
	source := projectDir(t, filepath.Join(root, "Sources", "Feature"))
	found, err := FindCurrentProject(source)
	if err != nil || found != project {
		t.Fatalf("found=%s err=%v", found, err)
	}
	workspace := projectDir(t, filepath.Join(root, "App.xcworkspace"))
	found, err = FindCurrentProject(source)
	if err != nil || found != workspace {
		t.Fatalf("found=%s err=%v", found, err)
	}
	projectDir(t, filepath.Join(root, "Other.xcworkspace"))
	if _, err = FindCurrentProject(source); err == nil {
		t.Fatal("ambiguous workspaces accepted")
	}
	found, err = FindCurrentProject(project)
	if err != nil || found != project {
		t.Fatalf("bundle detection=%s err=%v", found, err)
	}
	inner := projectDir(t, filepath.Join(project, "project.xcworkspace"))
	found, err = FindCurrentProject(inner)
	if err != nil || found != project {
		t.Fatalf("internal workspace=%s err=%v", found, err)
	}
}
func TestCurrentMatcherUsesPathsNotNames(t *testing.T) {
	root := t.TempDir()
	project := projectDir(t, filepath.Join(root, "one", "App.xcodeproj"))
	other := projectDir(t, filepath.Join(root, "two", "App.xcodeproj"))
	for _, tc := range []struct {
		name, path string
		want       bool
	}{{"match", project, true}, {"same-name", other, false}, {"internal", projectDir(t, filepath.Join(project, "project.xcworkspace")), true}, {"missing", filepath.Join(root, "gone.xcodeproj"), false}} {
		entry := projectDir(t, filepath.Join(root, tc.name))
		if err := os.WriteFile(filepath.Join(entry, "info.plist"), []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
		match := currentProjectMatcher(project, func(context.Context, string) ([]byte, error) {
			return []byte(fmt.Sprintf("{\"WorkspacePath\":%q}", tc.path)), nil
		})
		got, err := match(context.Background(), entry)
		if err != nil || got != tc.want {
			t.Fatalf("%s got=%v err=%v", tc.name, got, err)
		}
	}
	missing := projectDir(t, filepath.Join(root, "no-metadata"))
	got, err := CurrentProjectMatcher(project)(context.Background(), missing)
	if err != nil || got {
		t.Fatalf("missing metadata=%v %v", got, err)
	}
}
func TestCurrentMatcherReadsXMLAndBinaryPlists(t *testing.T) {
	project := projectDir(t, filepath.Join(t.TempDir(), "App.xcodeproj"))
	for _, format := range []string{"xml1", "binary1"} {
		entry := projectDir(t, filepath.Join(t.TempDir(), "App-hash"))
		path := filepath.Join(entry, "info.plist")
		xml := fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict><key>WorkspacePath</key><string>%s</string></dict></plist>`, project)
		if err := os.WriteFile(path, []byte(xml), 0644); err != nil {
			t.Fatal(err)
		}
		if format == "binary1" {
			if _, err := exec.Command("/usr/bin/plutil", "-convert", format, path).CombinedOutput(); err != nil {
				t.Fatal(err)
			}
		}
		got, err := CurrentProjectMatcher(project)(context.Background(), entry)
		if err != nil || !got {
			t.Fatalf("%s got=%v err=%v", format, got, err)
		}
	}
}
func TestMetadataFilterRunsBeforeStats(t *testing.T) {
	target := fixtureTarget(t)
	calls := 0
	p, err := scanTarget(context.Background(), target, ScanOptions{MatchEntry: func(ctx context.Context, path string) (bool, error) { return filepath.Base(path) == "B", nil }}, nil, func(ctx context.Context, path string, activity bool) DerivedEntry {
		calls++
		return scanEntry(ctx, path, activity)
	})
	if err != nil || calls != 1 || len(p.Entries) != 1 || p.Entries[0].Name != "B" {
		t.Fatalf("calls=%d plan=%+v err=%v", calls, p, err)
	}
}
