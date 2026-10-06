package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type TargetChoice struct{ Name, Label, Description string }

type targetMenu struct {
	choices               []TargetChoice
	cursor, width, height int
	selected              string
	title                 string
}

func (m *targetMenu) Init() tea.Cmd { return nil }
func (m *targetMenu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(m.choices)-1, m.cursor+1)
		case "enter":
			if len(m.choices) > 0 {
				m.selected = m.choices[m.cursor].Name
			}
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m *targetMenu) View() string {
	rows := max(1, m.height-3)
	start := max(0, m.cursor-rows+1)
	title := m.title
	if title == "" {
		title = "Choose a cleanup target"
	}
	lines := []string{ansi.Truncate(title, m.width, "…")}
	for i := start; i < min(len(m.choices), start+rows); i++ {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		choice := m.choices[i]
		lines = append(lines, ansi.Truncate(fmt.Sprintf("%s%s — %s", prefix, choice.Label, choice.Description), m.width, "…"))
	}
	lines = append(lines, ansi.Truncate("↑/↓ or j/k move • Enter continue • q/Esc cancel", m.width, "…"))
	return strings.Join(lines, "\n") + "\n"
}

func RunTargetMenu(ctx context.Context, out io.Writer, in io.Reader, choices []TargetChoice) (string, error) {
	return RunChoiceMenu(ctx, out, in, "Choose a cleanup target", choices)
}

// RunChoiceMenu presents an inline single-choice menu without clearing scrollback.
func RunChoiceMenu(ctx context.Context, out io.Writer, in io.Reader, title string, choices []TargetChoice) (string, error) {
	model := &targetMenu{choices: choices, title: title, width: 80, height: 24}
	final, err := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithContext(ctx)).Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	return final.(*targetMenu).selected, nil
}
