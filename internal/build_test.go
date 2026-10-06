package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func buildFixture(t *testing.T) Target {
	t.Helper()
	target := Target{Name: "DerivedData", Path: t.TempDir()}
	for path, data := range map[string]string{"App/Build/Products/product": "123", "App/Build/Intermediates.noindex/object": "12345", "App/Build/Other/keep": "other", "App/Index.noindex/index": "index", "App/SourcePackages/checkouts/file": "checkout", "App/Logs/log": "log", "App/info.plist": "metadata", "ModuleCache.noindex/module": "module"} {
		absolute := filepath.Join(target.Path, path)
		if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return target
}
func TestBuildOnlyPreservesProjectCaches(t *testing.T) {
	target := buildFixture(t)
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true, Activity: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	size, count := p.Summary()
	if size != 8 || count != 1 {
		t.Fatalf("size=%d count=%d entries=%+v", size, count, p.Entries)
	}
	result, err := DeletePlan(context.Background(), p, nil)
	if err != nil || result.Bytes != 8 || result.Items != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, path := range []string{"App/Build/Products", "App/Build/Intermediates.noindex"} {
		if _, err := os.Stat(filepath.Join(target.Path, path)); !os.IsNotExist(err) {
			t.Fatal("output remains", path, err)
		}
	}
	for path, want := range map[string]string{"App/Build/Other/keep": "other", "App/Index.noindex/index": "index", "App/SourcePackages/checkouts/file": "checkout", "App/Logs/log": "log", "App/info.plist": "metadata", "ModuleCache.noindex/module": "module"} {
		data, err := os.ReadFile(filepath.Join(target.Path, path))
		if err != nil || string(data) != want {
			t.Fatal("preserved file changed", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target.Path, "App", "Build")); err != nil {
		t.Fatal("Build parent removed", err)
	}
}
func TestBuildOnlyPartialDeletionAccounting(t *testing.T) {
	target := buildFixture(t)
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("permission denied")
	result, err := deletePlan(context.Background(), p, nil, func(root *os.Root, name string) error {
		if filepath.Base(name) == "Products" {
			return failure
		}
		return root.RemoveAll(name)
	})
	if !errors.Is(err, failure) || result.Bytes != 5 || result.Items != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "App", "Build", "Products")); err != nil {
		t.Fatal(err)
	}
}
func TestBuildOnlySkipsReplacedOutputs(t *testing.T) {
	target := buildFixture(t)
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(target.Path, "App", "Build", "Products")
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	result, err := DeletePlan(context.Background(), p, nil)
	if err == nil || result.Bytes != 5 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("replacement removed", err)
	}
	if _, err := os.Stat(path + "-old"); err != nil {
		t.Fatal("unconfirmed directory removed", err)
	}
}
func TestBuildOnlyNeverFollowsSymlinks(t *testing.T) {
	target := buildFixture(t)
	outside := t.TempDir()
	file := filepath.Join(outside, "keep")
	if err := os.WriteFile(file, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target.Path, "LinkedProject")); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(target.Path, "LinkedBuild")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(project, "Build")); err != nil {
		t.Fatal(err)
	}
	product := filepath.Join(target.Path, "App", "Build", "Products")
	if err := os.RemoveAll(product); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, product); err != nil {
		t.Fatal(err)
	}
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := DeletePlan(context.Background(), p, nil)
	if err != nil || result.Bytes != 5 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Lstat(product); err != nil {
		t.Fatal("output symlink removed", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("symlink target changed", err)
	}
}
func TestBuildOnlyAgeIgnoresPreservedCaches(t *testing.T) {
	target := buildFixture(t)
	old := time.Now().Add(-90 * 24 * time.Hour)
	for _, path := range []string{"App/Build/Products/product", "App/Build/Intermediates.noindex/object"} {
		if err := os.Chtimes(filepath.Join(target.Path, path), old, old); err != nil {
			t.Fatal(err)
		}
	}
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true, OlderThan: "30d"}, nil)
	if err != nil || len(p.Entries) != 1 {
		t.Fatalf("entries=%+v error=%v", p.Entries, err)
	}
}
func TestBuildOnlyNoBuildOutputs(t *testing.T) {
	target := Target{Path: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(target.Path, "App", "Index.noindex"), 0755); err != nil {
		t.Fatal(err)
	}
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true}, nil)
	if err != nil || len(p.Entries) != 0 {
		t.Fatalf("entries=%+v err=%v", p.Entries, err)
	}
}

func TestBuildOnlyRejectsReplacedBuildParent(t *testing.T) {
	target := buildFixture(t)
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(target.Path, "App", "Build")
	if err := os.Rename(build, build+"-old"); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	keep := filepath.Join(outside, "Products", "keep")
	if err := os.MkdirAll(filepath.Dir(keep), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, build); err != nil {
		t.Fatal(err)
	}
	result, err := DeletePlan(context.Background(), p, nil)
	if err == nil || result.Bytes != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("external data removed", err)
	}
	if _, err := os.Stat(filepath.Join(build+"-old", "Products", "product")); err != nil {
		t.Fatal("unconfirmed data removed", err)
	}
}
func TestBuildOnlyCancellationBetweenOutputs(t *testing.T) {
	target := buildFixture(t)
	p, err := ScanTarget(context.Background(), target, ScanOptions{BuildOnly: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := deletePlan(ctx, p, nil, func(root *os.Root, name string) error { err := root.RemoveAll(name); cancel(); return err })
	if !errors.Is(err, context.Canceled) || result.Bytes != 3 || result.Items != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "App", "Build", "Intermediates.noindex", "object")); err != nil {
		t.Fatal("canceled output removed", err)
	}
}
