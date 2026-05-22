package tui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestIsInteractiveTerminalReturnsFalseForNonFileIO(t *testing.T) {
	if IsInteractiveTerminal(strings.NewReader("input"), &bytes.Buffer{}) {
		t.Fatal("IsInteractiveTerminal() = true, want false for non-file input/output")
	}
}

func TestIsInteractiveTerminalReturnsFalseForPipeOutput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsInteractiveTerminal(r, w) {
		t.Fatal("IsInteractiveTerminal() = true, want false for pipe input/output")
	}
}
