//go:build unix

package internal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunPackageToolStreamsAndStopsChildren(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "xcrun")
	// A descendant keeps output descriptors open; group cancellation must close them.
	contents := "#!/bin/sh\necho 'Fetching fixture'\nsleep 30 &\nwait\n"
	if err := os.WriteFile(script, []byte(contents), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out bytes.Buffer
	start := time.Now()
	err := RunPackageTool(ctx, &out, "swift", "--version")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(out.String(), "Fetching fixture") {
		t.Fatalf("%s %v", out.String(), err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("child retained output pipes after cancellation")
	}
}
