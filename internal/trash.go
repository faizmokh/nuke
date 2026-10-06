package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// TrashPlan moves confirmed entries into the user's Trash. Bytes describes the
// logical size moved, not space freed. No permanent-deletion fallback is used.
func TrashPlan(ctx context.Context, plan ScanPlan, onProgress func(int, int)) (DeleteResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DeleteResult{}, err
	}
	return trashPlanAt(ctx, plan, filepath.Join(home, ".Trash"), onProgress)
}

func trashPlanAt(ctx context.Context, plan ScanPlan, destination string, onProgress func(int, int)) (DeleteResult, error) {
	if err := ctx.Err(); err != nil {
		return DeleteResult{}, err
	}
	if !trashSupported {
		return DeleteResult{}, errors.New("moving to Trash is supported only on macOS")
	}
	for _, entry := range plan.Entries {
		if entry.buildOnly {
			return DeleteResult{}, errors.New("Trash does not support nested build outputs")
		}
	}
	// Reject trashing the Trash itself or any of its contents.
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return DeleteResult{}, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		absolute = resolved
	} else if !os.IsNotExist(err) {
		return DeleteResult{}, err
	}
	rel, err := filepath.Rel(absolute, plan.rootPath)
	if err != nil {
		return DeleteResult{}, err
	}
	if rel == "." || (rel != ".." && !filepath.IsAbs(rel) && filepath.IsLocal(rel)) {
		return DeleteResult{}, errors.New("cannot move Trash contents into Trash")
	}
	var trash *os.Root
	defer func() {
		if trash != nil {
			trash.Close()
		}
	}()
	return deletePlan(ctx, plan, onProgress, func(source *os.Root, name string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if trash == nil {
			if err := os.Mkdir(destination, 0700); err != nil && !os.IsExist(err) {
				return fmt.Errorf("creating Trash: %w", err)
			}
			var err error
			trash, err = os.OpenRoot(destination)
			if err != nil {
				return fmt.Errorf("opening Trash: %w", err)
			}
		}
		if err := moveToTrash(source, trash, name); err != nil {
			return fmt.Errorf("moving to Trash (archive left in place): %w", err)
		}
		return nil
	})
}
