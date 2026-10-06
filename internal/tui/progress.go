package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type InlineProgress struct {
	w          io.Writer
	label      string
	total      int
	model      progress.Model
	style      lipgloss.Style
	active     bool
	lastUpdate time.Time
	now        func() time.Time
	width      int
}

func NewInlineProgress(w io.Writer, label string, total int) *InlineProgress {
	width := terminalWidth(w)
	label = ansi.Truncate(label, max(1, width/3), "…")
	return &InlineProgress{
		width:  width,
		w:      w,
		label:  label,
		total:  total,
		model:  progress.New(progress.WithDefaultGradient(), progress.WithWidth(max(1, width-ansi.StringWidth(label)-16))),
		style:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		active: total > 0,
		now:    time.Now,
	}
}

func (p *InlineProgress) Update(current int) {
	if !p.active {
		return
	}

	now := p.now()
	if current < p.total && !p.lastUpdate.IsZero() && now.Sub(p.lastUpdate) < 100*time.Millisecond {
		return
	}
	p.lastUpdate = now
	ratio := float64(current) / float64(p.total)
	if ratio > 1 {
		ratio = 1
	}

	if ratio < 0 {
		ratio = 0
	}
	bar := p.model.ViewAs(ratio)
	line := fmt.Sprintf("%s %s %d/%d", p.style.Render(p.label), bar, current, p.total)
	fmt.Fprintf(p.w, "\r%s", ansi.Truncate(line, p.width, "…"))
}

func (p *InlineProgress) Done() {
	if !p.active {
		return
	}
	fmt.Fprint(p.w, "\n")
}
