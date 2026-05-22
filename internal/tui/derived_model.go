package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faizmokh/nuke/internal"
)

type derivedRow struct {
	entry    internal.DerivedEntry
	loaded   bool
	selected bool
}

type DerivedModel struct {
	target        internal.Target
	spinner       spinner.Model
	rows          []derivedRow
	cursor        int
	scanDone      int
	scanTotal     int
	scanFinished  bool
	scanErr       error
	confirmed     bool
	cancelled     bool
	statusMessage string
}

func NewDerivedModel(target internal.Target) *DerivedModel {
	s := spinner.New()
	s.Spinner = spinner.Line

	return &DerivedModel{
		target:  target,
		spinner: s,
	}
}

func (m *DerivedModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *DerivedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case internal.DerivedScanUpdate:
		m.applyScanUpdate(msg)
		return m, nil
	case scanFinishedMsg:
		m.scanFinished = true
		m.scanErr = msg.err
		if msg.err != nil {
			m.statusMessage = msg.err.Error()
			return m, tea.Quit
		}
		m.scanDone = m.scanTotal
		if len(m.rows) == 0 {
			return m, tea.Quit
		}
		if m.scanTotal == 0 {
			m.scanTotal = len(m.rows)
			m.scanDone = len(m.rows)
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m *DerivedModel) View() string {
	headerStyle := lipgloss.NewStyle().Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	progress := fmt.Sprintf("%d/%d scanned", m.scanDone, m.scanTotal)
	if m.scanTotal == 0 {
		progress = "Scanning..."
	}

	lines := []string{
		headerStyle.Render(fmt.Sprintf("%s %s", m.target.Name, progress)),
	}

	if len(m.rows) == 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%s Scanning entries...", m.spinner.View())))
		return strings.Join(lines, "\n")
	}

	for i, row := range m.rows {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}

		check := "[ ]"
		if row.selected {
			check = "[x]"
		}

		details := "Scanning..."
		if row.loaded {
			details = fmt.Sprintf("%8s  %s", internal.HumanSize(row.entry.Size), row.entry.LastActivity.Format("2006-01-02"))
		}

		lines = append(lines, fmt.Sprintf("%s %s %-24s %s", cursor, check, row.entry.Name, details))
	}

	if m.statusMessage != "" {
		lines = append(lines, "", statusStyle.Render(m.statusMessage))
	}

	lines = append(lines, "", mutedStyle.Render("j/k or arrows move • space select • enter confirm • q cancel"))
	return strings.Join(lines, "\n")
}

func (m *DerivedModel) Selection() []internal.DerivedEntry {
	if m.cancelled {
		return nil
	}

	selected := make([]internal.DerivedEntry, 0, len(m.rows))
	for _, row := range m.rows {
		if row.selected {
			selected = append(selected, row.entry)
		}
	}
	return selected
}

func (m *DerivedModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		return m, nil
	case tea.KeySpace:
		if len(m.rows) > 0 {
			m.rows[m.cursor].selected = !m.rows[m.cursor].selected
			m.statusMessage = ""
		}
		return m, nil
	case tea.KeyEnter:
		if !m.scanFinished {
			m.statusMessage = "Scanning must finish before deletion can continue."
			return m, nil
		}
		m.confirmed = true
		m.statusMessage = ""
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyCtrlC:
		m.cancelled = true
		return m, tea.Quit
	}

	switch msg.String() {
	case "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "a":
		for i := range m.rows {
			m.rows[i].selected = true
		}
		m.statusMessage = ""
	case "n":
		for i := range m.rows {
			m.rows[i].selected = false
		}
		m.statusMessage = ""
	case "q":
		m.cancelled = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *DerivedModel) applyScanUpdate(update internal.DerivedScanUpdate) {
	if update.Total > m.scanTotal {
		m.scanTotal = update.Total
	}
	if update.Done > m.scanDone {
		m.scanDone = update.Done
	}

	for len(m.rows) <= update.Index {
		m.rows = append(m.rows, derivedRow{})
	}

	row := m.rows[update.Index]
	row.entry = update.Entry
	row.loaded = update.Complete

	m.rows[update.Index] = row
	sort.SliceStable(m.rows, func(i, j int) bool {
		return m.rows[i].entry.Name < m.rows[j].entry.Name
	})

	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
