package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
		Index:    0,
		Entry:    internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Complete: true,
		Total:    1,
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
		Index:    0,
		Entry:    internal.DerivedEntry{Name: "MyApp-abc123", Path: "/tmp/MyApp-abc123"},
		Complete: true,
		Total:    1,
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

func TestFilteredOutOfOrderRowsPreserveSelection(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	// Original scan indexes are sparse after filtering; paths identify entries.
	for _, u := range []internal.DerivedScanUpdate{
		{Index: 9, Entry: internal.DerivedEntry{Name: "Z", Path: "/tmp/Z"}, Total: 10},
		{Index: 3, Entry: internal.DerivedEntry{Name: "D", Path: "/tmp/D"}, Total: 10},
		{Index: 9, Entry: internal.DerivedEntry{Name: "Z", Path: "/tmp/Z", Size: 9}, Total: 10, Complete: true},
	} {
		m.Update(u)
	}
	if len(m.rows) != 2 {
		t.Fatalf("rows=%+v", m.rows)
	}
	m.cursor = 1
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(internal.DerivedScanUpdate{Index: 3, Entry: internal.DerivedEntry{Name: "D", Path: "/tmp/D", Size: 3}, Complete: true, Total: 10})
	selected := m.Selection()
	if len(selected) != 1 || selected[0].Name != "Z" || selected[0].Size != 9 {
		t.Fatalf("selection=%+v", selected)
	}
	if !strings.Contains(m.View(), "Selected: 1") {
		t.Fatal(m.View())
	}
}

func TestFailedAndLoadingRowsCannotBeSelected(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(internal.ScanUpdate{Entry: internal.DerivedEntry{Name: "A", Path: "/tmp/A"}})
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(m.Selection()) != 0 {
		t.Fatal("loading entry selected")
	}
	m.Update(internal.ScanUpdate{Entry: internal.DerivedEntry{Name: "A", Path: "/tmp/A", Err: errors.New("permission denied")}, Complete: true})
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(m.Selection()) != 0 {
		t.Fatal("failed entry selected")
	}
	if !strings.Contains(m.View(), "Unavailable") {
		t.Fatal(m.View())
	}
	m.Update(scanFinishedMsg{err: errors.New("partial scan")})
	if m.confirmed {
		t.Fatal("scan error auto confirmed")
	}
}

func TestViewportFitsAndScrolls(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 10})
	for i := 0; i < 50; i++ {
		m.Update(internal.ScanUpdate{Entry: internal.DerivedEntry{Name: fmt.Sprintf("%03d-very-long-project-name-世界-world", i), Path: fmt.Sprintf("/tmp/%d", i)}, Complete: true})
	}
	m.cursor = 49
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > 10 {
		t.Fatalf("too many lines: %d", len(lines))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > 36 {
			t.Fatalf("line too wide: %q", line)
		}
	}
	if !strings.Contains(view, "049-") || strings.Contains(view, "000-") {
		t.Fatal(view)
	}
	m.Update(tea.WindowSizeMsg{Width: 1, Height: 1})
	view = m.View()
	if strings.Contains(view, "\n") || ansi.StringWidth(view) > 1 {
		t.Fatalf("tiny terminal view=%q", view)
	}
}

func TestSelectAllIncludesLaterSuccessfulRows(t *testing.T) {
	m := NewDerivedModel(internal.Target{Name: "DerivedData"})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m.Update(internal.ScanUpdate{Entry: internal.DerivedEntry{Name: "A", Path: "/tmp/A", Size: 5}, Complete: true})
	if len(m.Selection()) != 1 {
		t.Fatal("select all omitted late entry")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(m.Selection()) != 0 {
		t.Fatal("select none failed")
	}
}
