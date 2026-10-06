package tui

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestTargetMenuNavigationSelectionAndCancel(t *testing.T) {
	choices := []TargetChoice{{Name: "derived", Label: "DerivedData"}, {Name: "spm", Label: "SwiftPM"}}
	m := &targetMenu{choices: choices, width: 80, height: 24}
	key := func(k tea.KeyType) { m.Update(tea.KeyMsg{Type: k}) }
	key(tea.KeyUp)
	if m.cursor != 0 {
		t.Fatal(m.cursor)
	}
	key(tea.KeyDown)
	key(tea.KeyDown)
	if m.cursor != 1 {
		t.Fatal(m.cursor)
	}
	if !strings.Contains(m.View(), "> SwiftPM") {
		t.Fatal(m.View())
	}
	key(tea.KeyEnter)
	if m.selected != "spm" {
		t.Fatal(m.selected)
	}
	for _, k := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyCtrlC}, {Type: tea.KeyRunes, Runes: []rune{'q'}}} {
		m := &targetMenu{choices: choices}
		_, quit := m.Update(k)
		if quit == nil || m.selected != "" {
			t.Fatal("cancel selected a target")
		}
	}
}

func TestTargetMenuFitsWindow(t *testing.T) {
	m := &targetMenu{choices: []TargetChoice{{Label: "One"}, {Label: "Two"}, {Label: "Three"}, {Label: "Four"}}, cursor: 3}
	m.Update(tea.WindowSizeMsg{Width: 12, Height: 4})
	view := m.View()
	if !strings.Contains(view, "> Four") {
		t.Fatal(view)
	}
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if ansi.StringWidth(line) > 12 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
	if strings.Count(view, "\n") > 4 {
		t.Fatal(view)
	}
}

func TestTargetMenuCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunTargetMenu(ctx, io.Discard, strings.NewReader(""), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
