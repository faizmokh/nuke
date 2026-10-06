package internal

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindCurrentProject searches the nearest enclosing directory. Workspaces take
// precedence over projects; ambiguous choices are errors rather than guesses.
func FindCurrentProject(start string) (string, error) {
	path, err := filepath.EvalSymlinks(start)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for {
		if strings.HasSuffix(path, ".xcodeproj") || strings.HasSuffix(path, ".xcworkspace") {
			if strings.HasSuffix(filepath.Dir(path), ".xcodeproj") && filepath.Base(path) == "project.xcworkspace" {
				return filepath.Dir(path), nil
			}
			return path, nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}
		var workspaces, projects []string
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".xcworkspace") && !strings.HasSuffix(e.Name(), ".xcodeproj") {
				continue
			}
			info, err := os.Stat(filepath.Join(path, e.Name()))
			if err != nil {
				return "", err
			}
			if !info.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".xcworkspace") {
				workspaces = append(workspaces, filepath.Join(path, e.Name()))
			}
			if strings.HasSuffix(e.Name(), ".xcodeproj") {
				projects = append(projects, filepath.Join(path, e.Name()))
			}
		}
		candidates := workspaces
		if len(candidates) == 0 {
			candidates = projects
		}
		if len(candidates) == 1 {
			return filepath.EvalSymlinks(candidates[0])
		}
		if len(candidates) > 1 {
			return "", fmt.Errorf("multiple Xcode projects/workspaces found: %s; run from inside the intended bundle or use --project", strings.Join(candidates, ", "))
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return "", fmt.Errorf("no Xcode workspace or project found from %s", start)
}

// CurrentProjectMatcher reads Xcode's per-entry Info.plist before expensive
// traversal. XML is decoded in-process; binary plists use macOS plutil.
func CurrentProjectMatcher(project string) func(context.Context, string) (bool, error) {
	return currentProjectMatcher(project, func(ctx context.Context, path string) ([]byte, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(string(data), "bplist") {
			var document struct {
				XMLName xml.Name
				Dict    struct {
					Values []struct {
						XMLName xml.Name
						Text    string `xml:",chardata"`
					} `xml:",any"`
				} `xml:"dict"`
			}
			if err := xml.Unmarshal(data, &document); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", path, err)
			}
			if document.XMLName.Local != "plist" {
				return nil, fmt.Errorf("invalid property list: %s", path)
			}
			fields := map[string]string{}
			values := document.Dict.Values
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid property list dictionary: %s", path)
			}
			for i := 0; i+1 < len(values); i += 2 {
				if values[i].XMLName.Local != "key" {
					return nil, fmt.Errorf("invalid property list dictionary: %s", path)
				}
				key := values[i].Text
				if (key == "WorkspacePath" || key == "ProjectPath") && values[i+1].XMLName.Local == "string" {
					fields[key] = values[i+1].Text
				}
			}
			return json.Marshal(fields)
		}
		output, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", path).CombinedOutput()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w (%s)", path, err, output)
		}
		return output, nil
	})
}

func currentProjectMatcher(project string, read func(context.Context, string) ([]byte, error)) func(context.Context, string) (bool, error) {
	if resolved, err := filepath.EvalSymlinks(project); err == nil {
		project = resolved
	}
	return func(ctx context.Context, entry string) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		info, err := os.Lstat(entry)
		if err != nil {
			return false, err
		}
		if !info.IsDir() {
			return false, nil
		}
		plist := filepath.Join(entry, "info.plist")
		info, err = os.Lstat(plist)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, nil
		}
		data, err := read(ctx, plist)
		if err != nil {
			return false, err
		}
		var metadata struct {
			WorkspacePath string
			ProjectPath   string
		}
		if err = json.Unmarshal(data, &metadata); err != nil {
			return false, fmt.Errorf("parsing %s: %w", plist, err)
		}
		for _, candidate := range []string{metadata.WorkspacePath, metadata.ProjectPath} {
			if !filepath.IsAbs(candidate) {
				continue
			}
			resolved, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				continue
			}
			if resolved == project || (strings.HasSuffix(project, ".xcodeproj") && resolved == filepath.Join(project, "project.xcworkspace")) {
				return true, nil
			}
		}
		return false, nil
	}
}
