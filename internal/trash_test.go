//go:build darwin

package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func trashFixture(t *testing.T) (ScanPlan, string) {
	t.Helper()
	base := t.TempDir()
	source := filepath.Join(base, "Archives")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.xcarchive", "second.xcarchive"} {
		if err := os.Mkdir(filepath.Join(source, name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name, "data"), []byte("abc"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := ScanTarget(context.Background(), Target{Name: "Archives", Path: source}, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return plan, filepath.Join(base, "Trash")
}

func TestTrashPlanMovesOnlySelectedAndPreservesCollision(t *testing.T) {
	plan, trash := trashFixture(t)
	if err := os.Mkdir(trash, 0700); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(trash, plan.Entries[0].Name)
	if err := os.WriteFile(collision, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	selected := plan.Select(plan.Entries[:1])
	// A new sibling must never be swept into the operation.
	sibling := filepath.Join(plan.rootPath, "new.xcarchive")
	if err := os.Mkdir(sibling, 0700); err != nil {
		t.Fatal(err)
	}
	progress := 0
	result, err := trashPlanAt(context.Background(), selected, trash, func(done, total int) {
		progress = done
		if total != 1 {
			t.Fatal(total)
		}
	})
	if err != nil || result.Items != 1 || result.Bytes != 3 || progress != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	data, err := os.ReadFile(collision)
	if err != nil || string(data) != "existing" {
		t.Fatal("collision overwritten", err)
	}
	entries, err := os.ReadDir(trash)
	if err != nil || len(entries) != 2 {
		t.Fatal(entries, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			data, err := os.ReadFile(filepath.Join(trash, entry.Name(), "data"))
			if err != nil || string(data) != "abc" {
				t.Fatal("archive content lost", err)
			}
		}
	}
	if _, err := os.Stat(plan.Entries[0].Path); !os.IsNotExist(err) {
		t.Fatal("archive not moved", err)
	}
	for _, path := range []string{plan.rootPath, plan.Entries[1].Path, sibling} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(path, err)
		}
	}
}

func TestTrashPlanRejectsReplacementAndAggregatesFailure(t *testing.T) {
	plan, trash := trashFixture(t)
	original := plan.Entries[0].Path
	if err := os.Rename(original, original+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	result, err := trashPlanAt(context.Background(), plan, trash, nil)
	if err == nil || !strings.Contains(err.Error(), "entry changed") || result.Items != 1 || result.Bytes != 3 {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatal("replacement removed", err)
	}
}

func TestTrashPlanCancellationAndFailurePreserveSource(t *testing.T) {
	for _, mode := range []string{"cancelled", "destination-file", "root-replaced"} {
		t.Run(mode, func(t *testing.T) {
			plan, trash := trashFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "cancelled":
				cancel()
			case "destination-file":
				if err := os.WriteFile(trash, []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			case "root-replaced":
				if err := os.Rename(plan.rootPath, plan.rootPath+"-saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(plan.rootPath, 0700); err != nil {
					t.Fatal(err)
				}
			}
			result, err := trashPlanAt(ctx, plan, trash, nil)
			if err == nil || result.Items != 0 || result.Bytes != 0 {
				t.Fatalf("%+v %v", result, err)
			}
			source := plan.rootPath
			if mode == "root-replaced" {
				source += "-saved"
			}
			for _, entry := range plan.Entries {
				if _, err := os.Stat(filepath.Join(source, entry.Name, "data")); err != nil {
					t.Fatal("source lost", err)
				}
			}
		})
	}
}

func TestTrashPlanCancellationBetweenMoves(t *testing.T) {
	plan, trash := trashFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := trashPlanAt(ctx, plan, trash, func(int, int) { cancel() })
	if err == nil || result.Items != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := os.Stat(plan.Entries[1].Path); err != nil {
		t.Fatal(err)
	}
}

func TestTrashPlanMovesSymlinkWithoutTouchingDestination(t *testing.T) {
	plan, trash := trashFixture(t)
	outside := t.TempDir()
	link := filepath.Join(plan.rootPath, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	plan, err := ScanTarget(context.Background(), plan.Target, ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var selected []DerivedEntry
	for _, e := range plan.Entries {
		if e.Name == "link" {
			selected = append(selected, e)
		}
	}
	result, err := trashPlanAt(context.Background(), plan.Select(selected), trash, nil)
	if err != nil || result.Items != 1 {
		t.Fatal(result, err)
	}
	destination, err := os.Readlink(filepath.Join(trash, "link"))
	if err != nil || destination != outside {
		t.Fatal(destination, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}
