package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSummaryFitsNarrowTerminal(t *testing.T) {
	for _, width := range []int{1, 4, 20, 36, 80} {
		view := renderSummaryCard(width, "Cleanup Complete", "Target: a very long project name 世界世界世界", "Freed (estimated): 100 MB")
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("width=%d actual=%d line=%q", width, ansi.StringWidth(line), line)
			}
		}
	}
}
