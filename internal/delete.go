package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type EntryResult struct {
	Entry   DerivedEntry
	Deleted bool
	Err     error
}

type DeleteResult struct {
	Bytes   int64 // Estimated size successfully removed from the target; Trash moves do not free space.
	Items   int
	Entries []EntryResult
}

func DeletePlan(ctx context.Context, plan ScanPlan, onProgress func(int, int)) (DeleteResult, error) {
	return deletePlan(ctx, plan, onProgress, func(root *os.Root, name string) error { return root.RemoveAll(name) })
}

func deletePlan(ctx context.Context, plan ScanPlan, onProgress func(int, int), remove func(*os.Root, string) error) (DeleteResult, error) {
	var result DeleteResult
	if err := ctx.Err(); err != nil {
		return result, err
	}
	root, err := os.OpenRoot(plan.rootPath)
	if err != nil {
		return result, fmt.Errorf("opening %s: %w", plan.Target.Name, err)
	}
	defer root.Close()
	openedInfo, err := root.Stat(".")
	if err != nil {
		return result, err
	}
	if plan.rootInfo == nil || !os.SameFile(plan.rootInfo, openedInfo) {
		return result, errors.New("target changed since scan")
	}
	var errs []error
	for i, e := range plan.Entries {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		er := EntryResult{Entry: e}
		switch {
		case e.Err != nil:
			er.Err = e.Err
		case filepath.Dir(e.Path) != plan.rootPath || filepath.Base(e.Path) != e.Name || e.Name == "." || e.Name == "..":
			er.Err = errors.New("entry is outside the confirmed target")
		default:
			// Check the configured target still resolves to the directory we scanned.
			currentPath, pathErr := filepath.EvalSymlinks(ExpandHome(plan.Target.Path))
			if pathErr != nil {
				er.Err = pathErr
			} else {
				currentPath, pathErr = filepath.Abs(currentPath)
				currentRoot, rootErr := os.Stat(currentPath)
				if pathErr != nil {
					er.Err = pathErr
				} else if rootErr != nil {
					er.Err = rootErr
				} else if currentPath != plan.rootPath || !os.SameFile(plan.rootInfo, currentRoot) {
					er.Err = errors.New("target changed since scan")
				}
			}
			if er.Err == nil {
				info, statErr := root.Lstat(e.Name)
				if statErr != nil {
					er.Err = statErr
				} else if !sameEntry(e.identity, info) {
					er.Err = errors.New("entry changed since scan")
				} else {
					if e.buildOnly {
						var freed int64
						freed, er.Err = deleteBuildParts(ctx, root, e, remove)
						result.Bytes += freed
					} else {
						er.Err = remove(root, e.Name)
					}
					if er.Err == nil {
						er.Deleted = true
						if !e.buildOnly {
							result.Bytes += e.Size
						}
						result.Items++
					}
				}
			}
		}
		if er.Err != nil {
			errs = append(errs, fmt.Errorf("skipping %s: %w", e.Name, er.Err))
		}
		result.Entries = append(result.Entries, er)
		if onProgress != nil {
			onProgress(i+1, len(plan.Entries))
		}
	}
	return result, errors.Join(append(errs, ctx.Err())...)
}
