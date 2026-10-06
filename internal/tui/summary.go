package tui

import (
	"github.com/charmbracelet/x/ansi"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderSummaryCard(title string, lines ...string) string {
	return renderSummaryCard(80, title, lines...)
}
func RenderSummaryCardFor(out io.Writer, title string, lines ...string) string {
	return renderSummaryCard(terminalWidth(out), title, lines...)
}
func renderSummaryCard(width int, title string, lines ...string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cardStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)

	body := make([]string, 0, len(lines)+1)
	body = append(body, titleStyle.Render(title))
	for _, line := range lines {
		body = append(body, bodyStyle.Render(line))
	}

	if width < 5 {
		for i, line := range body {
			body[i] = ansi.Truncate(line, max(1, width), "…")
		}
		return strings.Join(body, "\n")
	}
	return cardStyle.Width(max(1, width-2)).Render(strings.Join(body, "\n"))
}

func RenderConfirmPrompt(prompt string) string { return renderConfirmPrompt(80, prompt) }
func RenderConfirmPromptFor(out io.Writer, prompt string) string {
	return renderConfirmPrompt(terminalWidth(out), prompt)
}
func renderConfirmPrompt(width int, prompt string) string {
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	return renderSummaryCard(
		width,
		"Confirm Cleanup",
		promptStyle.Render(prompt),
		helpStyle.Render("Press y then Enter to continue, anything else to cancel."),
	)
}
