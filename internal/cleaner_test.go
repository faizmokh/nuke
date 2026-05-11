package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()
	target := Target{Name: "test", Path: dir}

	sub := filepath.Join(dir, "project1")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("hello world"), 0644)

	bytes, items, err := Scan(target)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if items != 1 {
		t.Errorf("Scan() items = %d, want 1", items)
	}
	if bytes == 0 {
		t.Error("Scan() bytes = 0, want > 0")
	}
}

func TestScanEmpty(t *testing.T) {
	dir := t.TempDir()
	target := Target{Name: "test", Path: dir}

	bytes, items, err := Scan(target)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if items != 0 {
		t.Errorf("Scan() items = %d, want 0", items)
	}
	if bytes != 0 {
		t.Errorf("Scan() bytes = %d, want 0", bytes)
	}
}

func TestScanNonexistent(t *testing.T) {
	target := Target{Name: "test", Path: "/nonexistent/path"}
	_, _, err := Scan(target)
	if err == nil {
		t.Error("Scan() expected error for nonexistent path")
	}
}

func TestNuke(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "project1")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "top.txt"), []byte("top"), 0644)

	target := Target{Name: "test", Path: dir}
	freed, err := Nuke(target, nil)
	if err != nil {
		t.Fatalf("Nuke() error: %v", err)
	}
	if freed == 0 {
		t.Error("Nuke() freed = 0, want > 0")
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("Nuke() left %d entries, want 0", len(entries))
	}
}

func TestRunDryRun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	target := Target{Name: "test", Path: dir}
	var out bytes.Buffer
	in := strings.NewReader("")

	err := Run(&out, in, target, false, true)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(out.String(), "1 items") {
		t.Errorf("Run() output = %q, want items count", out.String())
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Error("Dry run should not delete files")
	}
}

func TestRunConfirmYes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	target := Target{Name: "test", Path: dir}
	var out bytes.Buffer
	in := strings.NewReader("y\n")

	err := Run(&out, in, target, false, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("Run() left %d entries after confirm, want 0", len(entries))
	}
}

func TestRunConfirmNo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	target := Target{Name: "test", Path: dir}
	var out bytes.Buffer
	in := strings.NewReader("n\n")

	err := Run(&out, in, target, false, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Error("Declined confirm should not delete files")
	}
}

func TestRunSkipConfirm(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	target := Target{Name: "test", Path: dir}
	var out bytes.Buffer
	in := strings.NewReader("")

	err := Run(&out, in, target, true, false)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("Run() with yes=true left %d entries, want 0", len(entries))
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"~/test", true},
		{"/absolute/path", false},
		{"relative", false},
	}

	for _, tt := range tests {
		result := ExpandHome(tt.input)
		if tt.want {
			if !filepath.IsAbs(result) {
				t.Errorf("ExpandHome(%q) = %q, want absolute path", tt.input, result)
			}
		} else {
			if result != tt.input {
				t.Errorf("ExpandHome(%q) = %q, want unchanged", tt.input, result)
			}
		}
	}
}
