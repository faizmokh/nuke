package internal

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressBarUpdate(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		current int
		want    string
	}{
		{"zero", 10, 0, "[░░░░░░░░░░░░░░░░░░░░] 0/10 items"},
		{"half", 10, 5, "[██████████░░░░░░░░░░] 5/10 items"},
		{"full", 10, 10, "[████████████████████] 10/10 items"},
		{"single", 1, 1, "[████████████████████] 1/1 items"},
		{"partial", 3, 1, "[██████░░░░░░░░░░░░░░] 1/3 items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := NewProgressBar(&buf, tt.total)
			p.Update(tt.current)

			got := strings.TrimPrefix(buf.String(), "\r")
			if got != tt.want {
				t.Errorf("Update(%d/%d) = %q, want %q", tt.current, tt.total, got, tt.want)
			}
		})
	}
}

func TestProgressBarZeroTotal(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressBar(&buf, 0)
	p.Update(0)
	if buf.String() != "" {
		t.Errorf("Update with total=0 should produce no output, got %q", buf.String())
	}
}

func TestProgressBarDone(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressBar(&buf, 10)
	p.Done()

	output := buf.String()
	if !strings.HasPrefix(output, "\r") {
		t.Error("Done() should start with \\r to return to line start")
	}
	if !strings.Contains(output, "\r") {
		t.Error("Done() should contain \\r to clear the line")
	}
}

func TestProgressBarProgression(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressBar(&buf, 3)

	p.Update(1)
	if !strings.Contains(buf.String(), "1/3 items") {
		t.Errorf("first update should show 1/3, got %q", buf.String())
	}

	buf.Reset()
	p.Update(2)
	if !strings.Contains(buf.String(), "2/3 items") {
		t.Errorf("second update should show 2/3, got %q", buf.String())
	}

	buf.Reset()
	p.Update(3)
	if !strings.Contains(buf.String(), "3/3 items") {
		t.Errorf("third update should show 3/3, got %q", buf.String())
	}
}
