package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/faizmokh/nuke/internal"
)

type derivedRow struct {
	entry            internal.DerivedEntry
	loaded, selected bool
}
type DerivedModel struct {
	target               internal.Target
	spinner              spinner.Model
	rows                 []derivedRow
	rowIndexes           map[string]int
	cursor               int
	width, height        int
	scanDone, scanTotal  int
	scanFinished         bool
	scanErr              error
	plan                 internal.ScanPlan
	confirmed, cancelled bool
	selectAll            bool
	statusMessage        string
}

func NewDerivedModel(target internal.Target) *DerivedModel {
	s := spinner.New()
	s.Spinner = spinner.Line
	return &DerivedModel{target: target, spinner: s, rowIndexes: map[string]int{}, width: 80, height: 24}
}
func (m *DerivedModel) Init() tea.Cmd { return m.spinner.Tick }
func (m *DerivedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
	case spinner.TickMsg:
		if m.scanFinished {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case internal.ScanUpdate:
		m.applyUpdate(msg)
	case internal.DerivedScanUpdate:
		m.applyUpdate(internal.ScanUpdate{Entry: msg.Entry, Done: msg.Done, Total: msg.Total, Complete: msg.Complete})
	case scanFinishedMsg:
		m.scanFinished = true
		m.scanErr = msg.err
		m.plan = msg.plan
		m.scanDone = m.scanTotal
		if msg.err != nil {
			m.statusMessage = "Some entries could not be scanned; unavailable entries cannot be selected."
		}
		if len(m.rows) == 0 {
			return m, tea.Quit
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}
func (m *DerivedModel) View() string {
	header := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	progress := fmt.Sprintf("%d/%d scanned", m.scanDone, m.scanTotal)
	if !m.scanFinished && m.scanTotal == 0 {
		progress = "Scanning..."
	}
	if m.scanFinished {
		progress = "Scan complete"
	}
	var size int64
	var count, pending int
	for _, r := range m.rows {
		if r.selected && r.entry.Err == nil {
			count++
			if r.loaded {
				size += r.entry.Size
			} else {
				pending++
			}
		}
	}
	selection := fmt.Sprintf("Selected: %d · estimated %s", count, internal.HumanSize(size))
	if pending > 0 {
		selection += fmt.Sprintf(" (%d scanning)", pending)
	}
	lines := []string{header.Render(fmt.Sprintf("%s · %s", m.target.Name, progress)), selection}
	if len(m.rows) == 0 {
		lines = append(lines, muted.Render(fmt.Sprintf("%s Scanning entries...", m.spinner.View())))
	} else {
		// Reserve space for the header, selection total, status, and keyboard hints.
		visible := max(1, m.height-6)
		start := max(0, min(m.cursor-visible+1, len(m.rows)-visible))
		end := min(len(m.rows), start+visible)
		for i := start; i < end; i++ {
			row := m.rows[i]
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			check := "[ ]"
			if row.selected {
				check = "[x]"
			}
			details := "Scanning..."
			if row.entry.Err != nil {
				details = "Unavailable"
				check = "[-]"
			} else if row.loaded {
				details = fmt.Sprintf("%8s  %s", internal.HumanSize(row.entry.Size), row.entry.LastActivity.Format("2006-01-02"))
			}
			nameWidth := max(1, m.width-7-ansi.StringWidth(details))
			name := ansi.Truncate(row.entry.Name, nameWidth, "…")
			name += strings.Repeat(" ", max(0, nameWidth-ansi.StringWidth(name)))
			lines = append(lines, fmt.Sprintf("%s %s %s %s", cursor, check, name, details))
		}
		lines = append(lines, muted.Render(fmt.Sprintf("Entries %d–%d of %d", start+1, end, len(m.rows))))
	}
	status := m.statusMessage
	if status == "" && len(m.rows) > 0 && m.rows[m.cursor].entry.Err != nil {
		status = m.rows[m.cursor].entry.Err.Error()
	}
	if status != "" {
		lines = append(lines, status)
	}
	lines = append(lines, muted.Render("↑/↓ j/k move · space select · a all · n none · enter continue · q cancel"))
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "…")
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}
func (m *DerivedModel) Selection() []internal.DerivedEntry {
	if m.cancelled {
		return nil
	}
	var selected []internal.DerivedEntry
	for _, r := range m.rows {
		if r.selected && r.loaded && r.entry.Err == nil {
			selected = append(selected, r.entry)
		}
	}
	return selected
}
func (m *DerivedModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(max(0, len(m.rows)-1), m.cursor+1)
	case " ":
		if len(m.rows) > 0 {
			row := &m.rows[m.cursor]
			if row.entry.Err == nil {
				row.selected = !row.selected
				m.statusMessage = ""
				m.selectAll = false
			} else {
				m.statusMessage = "Failed entries cannot be selected."
			}
		}
	case "a":
		m.selectAll = true
		for i := range m.rows {
			m.rows[i].selected = m.rows[i].entry.Err == nil
		}
		m.statusMessage = ""
	case "n":
		m.selectAll = false
		for i := range m.rows {
			m.rows[i].selected = false
		}
		m.statusMessage = ""
	case "enter":
		if !m.scanFinished {
			m.statusMessage = "Scanning must finish before deletion can continue."
			return m, nil
		}
		m.confirmed = true
		m.statusMessage = ""
		return m, tea.Quit
	case "esc", "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}
func (m *DerivedModel) applyUpdate(u internal.ScanUpdate) {
	m.scanTotal = max(m.scanTotal, u.Total)
	m.scanDone = max(m.scanDone, u.Done)
	key := u.Entry.Path
	if key == "" {
		key = u.Entry.Name
	}
	if index, ok := m.rowIndexes[key]; ok {
		row := &m.rows[index]
		row.entry = u.Entry
		row.loaded = u.Complete
		if row.entry.Err != nil {
			row.selected = false
		} else if m.selectAll && row.loaded {
			row.selected = true
		}
		return
	}
	// Insert once in display order, preserving the cursor's entry identity.
	index := sort.Search(len(m.rows), func(i int) bool { return m.rows[i].entry.Name >= u.Entry.Name })
	m.rows = append(m.rows, derivedRow{})
	copy(m.rows[index+1:], m.rows[index:])
	m.rows[index] = derivedRow{entry: u.Entry, loaded: u.Complete, selected: m.selectAll && u.Entry.Err == nil}
	if len(m.rows) > 1 && index <= m.cursor {
		m.cursor++
	}
	for i := index; i < len(m.rows); i++ {
		k := m.rows[i].entry.Path
		if k == "" {
			k = m.rows[i].entry.Name
		}
		m.rowIndexes[k] = i
	}
}
