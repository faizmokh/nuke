package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageSetup identifies the container that owns dependency resolution.
type PackageSetup struct{ Kind, Path string }

func ValidatePackageSetup(kind, path string) (PackageSetup, error) {
	path, err := filepath.Abs(ExpandHome(path))
	if err != nil {
		return PackageSetup{}, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return PackageSetup{}, fmt.Errorf("open %s: %w", kind, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return PackageSetup{}, err
	}
	if kind == "package" && filepath.Base(path) == "Package.swift" {
		path = filepath.Dir(path)
	} else if !info.IsDir() {
		return PackageSetup{}, fmt.Errorf("%s must be a directory", path)
	}
	marker := "Package.swift"
	switch kind {
	case "project":
		if !strings.HasSuffix(path, ".xcodeproj") {
			return PackageSetup{}, fmt.Errorf("--project requires an .xcodeproj directory")
		}
		marker = "project.pbxproj"
	case "workspace":
		if !strings.HasSuffix(path, ".xcworkspace") {
			return PackageSetup{}, fmt.Errorf("--workspace requires an .xcworkspace directory")
		}
		marker = "contents.xcworkspacedata"
	case "package":
	default:
		return PackageSetup{}, fmt.Errorf("unknown package setup %q", kind)
	}
	info, err = os.Stat(filepath.Join(path, marker))
	if err != nil {
		return PackageSetup{}, fmt.Errorf("%s is missing or inaccessible: %w; generate the project first if using Tuist or XcodeGen", marker, err)
	}
	if !info.Mode().IsRegular() {
		return PackageSetup{}, fmt.Errorf("%s must be a regular file", marker)
	}
	return PackageSetup{kind, path}, nil
}

// DiscoverPackageSetups searches ancestors, never recursively scanning a repository.
// At each level workspaces take precedence over projects, then standalone packages.
func DiscoverPackageSetups(ctx context.Context, start string) ([]PackageSetup, error) {
	path, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kind := ""
		if strings.HasSuffix(path, ".xcodeproj") {
			kind = "project"
		}
		if strings.HasSuffix(path, ".xcworkspace") {
			kind = "workspace"
		}
		if kind != "" {
			if filepath.Base(path) == "project.xcworkspace" && strings.HasSuffix(filepath.Dir(path), ".xcodeproj") {
				path = filepath.Dir(path)
				kind = "project"
			}
			setup, err := ValidatePackageSetup(kind, path)
			return []PackageSetup{setup}, err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		var workspaces, projects []PackageSetup
		for _, entry := range entries {
			kind := ""
			if strings.HasSuffix(entry.Name(), ".xcworkspace") {
				kind = "workspace"
			}
			if strings.HasSuffix(entry.Name(), ".xcodeproj") {
				kind = "project"
			}
			if kind == "" {
				continue
			}
			candidate := filepath.Join(path, entry.Name())
			info, err := os.Stat(candidate)
			if err != nil {
				return nil, err
			}
			if !info.IsDir() {
				continue
			}
			setup := PackageSetup{Kind: kind, Path: candidate}
			if kind == "workspace" {
				workspaces = append(workspaces, setup)
			} else {
				projects = append(projects, setup)
			}
		}
		candidates := workspaces
		if len(candidates) == 0 {
			candidates = projects
		}
		if len(candidates) > 0 {
			var result []PackageSetup
			seen := map[string]bool{}
			for _, candidate := range candidates {
				setup, err := ValidatePackageSetup(candidate.Kind, candidate.Path)
				if err != nil {
					return nil, err
				}
				if !seen[setup.Path] {
					result = append(result, setup)
					seen[setup.Path] = true
				}
			}
			return result, nil
		}
		if _, err := os.Stat(filepath.Join(path, "Package.swift")); err == nil {
			setup, err := ValidatePackageSetup("package", path)
			return []PackageSetup{setup}, err
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		for _, marker := range []string{"Project.swift", "project.yml", "Tuist.swift"} {
			if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
				return nil, fmt.Errorf("no generated Xcode project in %s; run your Tuist/XcodeGen generation command first", path)
			}
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return nil, fmt.Errorf("no Xcode workspace, project, or Package.swift found; use --workspace, --project, or --package")
}

func (s PackageSetup) ResolvedPath() string {
	switch s.Kind {
	case "project":
		return filepath.Join(s.Path, "project.xcworkspace", "xcshareddata", "swiftpm", "Package.resolved")
	case "workspace":
		return filepath.Join(s.Path, "xcshareddata", "swiftpm", "Package.resolved")
	default:
		return filepath.Join(s.Path, "Package.resolved")
	}
}

func (s PackageSetup) DownloadArgs(scheme, derived string, locked, verbose bool) (string, []string) {
	if s.Kind == "package" {
		args := []string{"package", "--package-path", s.Path}
		if locked {
			args = append(args, "--force-resolved-versions")
		}
		if verbose {
			args = append(args, "--verbose")
		}
		return "swift", append(args, "resolve")
	}
	args := []string{"-resolvePackageDependencies", "-" + s.Kind, s.Path, "-skipPackageUpdates"}
	if scheme != "" {
		args = append(args, "-scheme", scheme)
	}
	if derived != "" {
		args = append(args, "-derivedDataPath", ExpandHome(derived))
	}
	if locked {
		args = append(args, "-onlyUsePackageVersionsFromResolvedFile")
	}
	if verbose {
		args = append(args, "-verbose")
	}
	return "xcodebuild", args
}
