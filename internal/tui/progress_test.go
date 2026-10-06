package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestInlineProgressThrottlesAndFlushesFinal(t *testing.T) {
	var out bytes.Buffer
	p := NewInlineProgress(&out, "Cleaning", 20)
	now := time.Unix(1, 0)
	p.now = func() time.Time { return now }
	p.Update(1)
	first := out.Len()
	for i := 2; i < 10; i++ {
		now = now.Add(time.Millisecond)
		p.Update(i)
	}
	if out.Len() != first {
		t.Fatal("progress was not throttled")
	}
	now = now.Add(100 * time.Millisecond)
	p.Update(10)
	if out.Len() == first {
		t.Fatal("next time window was not rendered")
	}
	p.Update(20)
	p.Done()
	if !strings.Contains(out.String(), "20/20") || !strings.HasSuffix(out.String(), "\n") {
		t.Fatal(out.String())
	}
}
