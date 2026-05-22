package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderSummaryCard(title string, lines ...string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cardStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)

	body := make([]string, 0, len(lines)+1)
	body = append(body, titleStyle.Render(title))
	for _, line := range lines {
		body = append(body, bodyStyle.Render(line))
	}

	return cardStyle.Render(strings.Join(body, "\n"))
}

func RenderConfirmPrompt(prompt string) string {
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	return RenderSummaryCard(
		"Confirm Cleanup",
		promptStyle.Render(prompt),
		helpStyle.Render("Press y then Enter to continue, anything else to cancel."),
	)
}
