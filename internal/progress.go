package internal

import (
	"fmt"
	"io"
	"strings"
)

type ProgressBar struct {
	w     io.Writer
	total int
	width int
}

func NewProgressBar(w io.Writer, total int) *ProgressBar {
	return &ProgressBar{w: w, total: total, width: 20}
}

func (p *ProgressBar) Update(current int) {
	if p.total == 0 {
		return
	}
	ratio := float64(current) / float64(p.total)
	filled := int(ratio * float64(p.width))
	if filled > p.width {
		filled = p.width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	fmt.Fprintf(p.w, "\r[%s] %d/%d items", bar, current, p.total)
}

func (p *ProgressBar) Done() {
	fmt.Fprintf(p.w, "\r%s\r", strings.Repeat(" ", p.width+30))
}
