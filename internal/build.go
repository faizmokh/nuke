package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Only these build outputs are eligible. Indexes, SourcePackages, logs, project
// metadata, and other children of Build remain untouched.
var buildDirectories = []string{"Products", "Intermediates.noindex"}

type buildPart struct {
	name     string
	size     int64
	identity os.FileInfo
}

func hasBuildOutputs(project string) (bool, error) {
	info, err := os.Lstat(project)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	info, err = os.Lstat(filepath.Join(project, "Build"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	for _, name := range buildDirectories {
		info, err := os.Lstat(filepath.Join(project, "Build", name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if info.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

func scanBuildEntry(ctx context.Context, path string, activity bool) DerivedEntry {
	e := DerivedEntry{Name: filepath.Base(path), Path: path, buildOnly: true}
	e.identity, e.Err = os.Lstat(path)
	if e.Err != nil {
		return e
	}
	if !e.identity.IsDir() {
		e.Err = errors.New("project entry is not a directory")
		return e
	}
	e.buildIdentity, e.Err = os.Lstat(filepath.Join(path, "Build"))
	if e.Err != nil {
		return e
	}
	if !e.buildIdentity.IsDir() {
		e.Err = errors.New("Build is not a directory")
		return e
	}
	for _, name := range buildDirectories {
		if err := ctx.Err(); err != nil {
			e.Err = err
			return e
		}
		partPath := filepath.Join(path, "Build", name)
		info, err := os.Lstat(partPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			e.Err = err
			return e
		}
		if !info.IsDir() {
			continue
		} // Never follow or remove output symlinks.
		part := scanEntry(ctx, partPath, activity)
		if part.Err != nil {
			e.Err = fmt.Errorf("Build/%s: %w", name, part.Err)
			return e
		}
		e.buildParts = append(e.buildParts, buildPart{name: name, size: part.Size, identity: part.identity})
		e.Size += part.Size
		if part.LastActivity.After(e.LastActivity) {
			e.LastActivity = part.LastActivity
		}
	}
	if len(e.buildParts) == 0 && e.Err == nil {
		e.Err = errors.New("build outputs disappeared during scan")
	}
	after, err := os.Lstat(path)
	if e.Err == nil && err != nil {
		e.Err = err
	} else if e.Err == nil && !sameEntry(e.identity, after) {
		e.Err = errors.New("project changed during scan")
	}
	after, err = os.Lstat(filepath.Join(path, "Build"))
	if e.Err == nil {
		if err != nil {
			e.Err = err
		} else if !sameEntry(e.buildIdentity, after) {
			e.Err = errors.New("Build changed during scan")
		}
	}
	return e
}

func deleteBuildParts(ctx context.Context, root *os.Root, e DerivedEntry, remove func(*os.Root, string) error) (int64, error) {
	var freed int64
	var errs []error
	for _, part := range e.buildParts {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		projectInfo, err := root.Lstat(e.Name)
		if err != nil || !sameEntry(e.identity, projectInfo) {
			if err == nil {
				err = errors.New("project changed since scan")
			}
			errs = append(errs, err)
			break
		}
		buildPath := filepath.Join(e.Name, "Build")
		info, err := root.Lstat(buildPath)
		if err == nil && (!info.IsDir() || !os.SameFile(e.buildIdentity, info) || info.Mode() != e.buildIdentity.Mode()) {
			err = errors.New("Build changed since scan")
		}
		path := filepath.Join(buildPath, part.name)
		if err == nil {
			info, err = root.Lstat(path)
			if err == nil && !sameEntry(part.identity, info) {
				err = errors.New("build output changed since scan")
			}
		}
		if err == nil {
			err = remove(root, path)
			if err == nil {
				freed += part.size
			}
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("Build/%s: %w", part.name, err))
		}
	}
	return freed, errors.Join(errs...)
}
