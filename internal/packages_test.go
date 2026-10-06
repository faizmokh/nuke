package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func packageFixture(t *testing.T, root, kind, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	marker := map[string]string{"project": "project.pbxproj", "workspace": "contents.xcworkspacedata", "package": "Package.swift"}[kind]
	if err := os.WriteFile(filepath.Join(path, marker), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func TestDiscoverPackageSetups(t *testing.T) {
	root := t.TempDir()
	project := packageFixture(t, root, "project", "App.xcodeproj")
	sub := filepath.Join(root, "Sources")
	os.Mkdir(sub, 0755)
	setups, err := DiscoverPackageSetups(context.Background(), sub)
	if err != nil || len(setups) != 1 || setups[0].Path != project {
		t.Fatalf("%v %v", setups, err)
	}
	workspace := packageFixture(t, root, "workspace", "App.xcworkspace")
	setups, err = DiscoverPackageSetups(context.Background(), sub)
	if err != nil || setups[0].Path != workspace {
		t.Fatalf("%v %v", setups, err)
	}
	packageFixture(t, root, "workspace", "Other.xcworkspace")
	setups, err = DiscoverPackageSetups(context.Background(), sub)
	if err != nil || len(setups) != 2 {
		t.Fatalf("%v %v", setups, err)
	}
	pkg := packageFixture(t, sub, "package", "Core")
	setups, err = DiscoverPackageSetups(context.Background(), pkg)
	if err != nil || setups[0].Kind != "package" {
		t.Fatalf("%v %v", setups, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DiscoverPackageSetups(ctx, sub); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	inner := packageFixture(t, project, "workspace", "project.xcworkspace")
	setups, err = DiscoverPackageSetups(context.Background(), inner)
	if err != nil || setups[0].Path != project {
		t.Fatalf("%v %v", setups, err)
	}
}
func TestPackageValidation(t *testing.T) {
	root := t.TempDir()
	pkg := packageFixture(t, root, "package", "Core")
	setup, err := ValidatePackageSetup("package", filepath.Join(pkg, "Package.swift"))
	if err != nil || setup.Path != pkg {
		t.Fatalf("%v %v", setup, err)
	}
	if _, err := ValidatePackageSetup("project", pkg); err == nil {
		t.Fatal("wrong suffix accepted")
	}
	if _, err := ValidatePackageSetup("workspace", filepath.Join(root, "Missing.xcworkspace")); err == nil {
		t.Fatal("missing accepted")
	}
	os.WriteFile(filepath.Join(root, "Project.swift"), nil, 0600)
	if _, err := DiscoverPackageSetups(context.Background(), root); err == nil || !strings.Contains(err.Error(), "generation") {
		t.Fatal(err)
	}
}
func TestPackageDownloadArguments(t *testing.T) {
	for _, kind := range []string{"project", "workspace", "package"} {
		s := PackageSetup{kind, "/tmp/path with spaces"}
		tool, args := s.DownloadArgs("", "", true, false)
		want := []string{"-resolvePackageDependencies", "-" + kind, s.Path, "-skipPackageUpdates", "-onlyUsePackageVersionsFromResolvedFile"}
		if kind == "package" {
			want = []string{"package", "--package-path", s.Path, "--force-resolved-versions", "resolve"}
			if tool != "swift" {
				t.Fatal(tool)
			}
		} else if tool != "xcodebuild" {
			t.Fatal(tool)
		}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("%v != %v", args, want)
		}
		_, args = s.DownloadArgs("", "", false, false)
		if strings.Contains(strings.Join(args, " "), "resolved") {
			t.Fatal(args)
		}
	}
}
func TestPackageLockReleased(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := t.TempDir()
	unlock, err := LockPackageSetup(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := LockPackageSetup(path); err == nil {
		second()
		t.Fatal("overlapping lock acquired")
	}
	unlock()
	unlock, err = LockPackageSetup(path)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestDiscoveryIgnoresLowerPriorityBrokenProjectAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	workspace := packageFixture(t, root, "workspace", "App.xcworkspace")
	os.Mkdir(filepath.Join(root, "Broken.xcodeproj"), 0755)
	if err := os.Symlink(workspace, filepath.Join(root, "Alias.xcworkspace")); err != nil {
		t.Fatal(err)
	}
	setups, err := DiscoverPackageSetups(context.Background(), root)
	if err != nil || len(setups) != 1 || setups[0].Path != workspace {
		t.Fatalf("%v %v", setups, err)
	}
}
