package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faizmokh/nuke/internal"
)

func TestDerivedModelShowsLoadingState(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})

	view := m.View()
	if view == "" {
		t.Fatal("View() = empty, want loading UI")
	}
	if !containsAll(view, "DerivedData", "Scanning") {
		t.Fatalf("View() = %q, want target name and scanning state", view)
	}
}

func TestDerivedModelUpdatesRowsAsScanCompletes(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(internal.DerivedScanUpdate{
		Index: 0,
		Entry: internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Total: 1,
	})

	loadingView := m.View()
	if !containsAll(loadingView, "MyApp-abc123", "Scanning...") {
		t.Fatalf("loading View() = %q, want placeholder row", loadingView)
	}

	updatedTime := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	m.Update(internal.DerivedScanUpdate{
		Index: 0,
		Entry: internal.DerivedEntry{
			Name:         "MyApp-abc123",
			Path:         "/tmp/MyApp-abc123",
			Size:         5 * 1024 * 1024,
			LastActivity: updatedTime,
		},
		Done:     1,
		Total:    1,
		Complete: true,
	})

	completedView := m.View()
	if !containsAll(completedView, "5.0 MB", "2025-01-10") {
		t.Fatalf("completed View() = %q, want completed row details", completedView)
	}
}

func TestDerivedModelTogglesSelection(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(internal.DerivedScanUpdate{
		Index: 0,
		Entry: internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Total: 1,
	})

	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	selected := m.Selection()
	if len(selected) != 1 || selected[0].Name != "MyApp-abc123" {
		t.Fatalf("Selection() = %#v, want selected first entry", selected)
	}

	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(m.Selection()) != 0 {
		t.Fatalf("Selection() = %#v, want empty after second toggle", m.Selection())
	}
}

func TestDerivedModelBlocksConfirmUntilScanCompletes(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(internal.DerivedScanUpdate{
		Index: 0,
		Entry: internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Total: 2,
	})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.confirmed {
		t.Fatal("confirmed = true, want false while scan is still running")
	}
	if m.statusMessage == "" {
		t.Fatal("statusMessage = empty, want feedback when confirm is blocked")
	}
}

func TestDerivedModelAllowsConfirmAfterScanFinishedMessage(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(internal.DerivedScanUpdate{
		Index: 0,
		Entry: internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Total: 2,
	})
	m.Update(internal.DerivedScanUpdate{
		Index:    0,
		Entry:    internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123", Size: 10, LastActivity: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)},
		Done:     1,
		Total:    2,
		Complete: true,
	})
	m.Update(scanFinishedMsg{})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.confirmed {
		t.Fatal("confirmed = false, want true after scan has finished")
	}
}

func TestDerivedModelCancelReturnsNoSelection(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(internal.DerivedScanUpdate{
		Index: 0,
		Entry: internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Total: 1,
	})
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if !m.cancelled {
		t.Fatal("cancelled = false, want true after cancel")
	}
	if len(m.Selection()) != 0 {
		t.Fatalf("Selection() = %#v, want empty after cancel", m.Selection())
	}
}

func containsAll(s string, substrings ...string) bool {
	for _, substring := range substrings {
		if !strings.Contains(s, substring) {
			return false
		}
	}
	return true
}
