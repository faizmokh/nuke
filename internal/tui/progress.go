package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

type InlineProgress struct {
	w      io.Writer
	label  string
	total  int
	model  progress.Model
	style  lipgloss.Style
	active bool
}

func NewInlineProgress(w io.Writer, label string, total int) *InlineProgress {
	return &InlineProgress{
		w:      w,
		label:  label,
		total:  total,
		model:  progress.New(progress.WithDefaultGradient()),
		style:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		active: total > 0,
	}
}

func (p *InlineProgress) Update(current int) {
	if !p.active {
		return
	}

	ratio := float64(current) / float64(p.total)
	if ratio > 1 {
		ratio = 1
	}

	bar := p.model.ViewAs(ratio)
	fmt.Fprintf(p.w, "\r%s %s %d/%d", p.style.Render(p.label), bar, current, p.total)
}

func (p *InlineProgress) Done() {
	if !p.active {
		return
	}
	fmt.Fprint(p.w, "\n")
}
