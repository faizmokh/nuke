package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/faizmokh/nuke/internal"
)

func TestPickerCancellationStopsScan(t *testing.T) {
	var out bytes.Buffer
	started := make(chan struct{})
	stopped := make(chan struct{})
	scan := func(ctx context.Context, target internal.Target, options internal.ScanOptions, onUpdate func(internal.ScanUpdate)) (internal.ScanPlan, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return internal.ScanPlan{}, ctx.Err()
	}
	result := make(chan error, 1)
	go func() {
		_, err := runDerivedPicker(context.Background(), &out, strings.NewReader("q"), internal.Target{Name: "test"}, internal.ScanOptions{}, scan)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("scan did not start")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("picker did not cancel")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("scan still running after picker returned")
	}
}

func TestPickerParentContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	started := make(chan struct{})
	scan := func(ctx context.Context, target internal.Target, options internal.ScanOptions, onUpdate func(internal.ScanUpdate)) (internal.ScanPlan, error) {
		close(started)
		<-ctx.Done()
		return internal.ScanPlan{}, ctx.Err()
	}
	result := make(chan error, 1)
	go func() {
		_, err := runDerivedPicker(ctx, io.Discard, reader, internal.Target{Name: "test"}, internal.ScanOptions{}, scan)
		result <- err
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("context cancellation stuck")
	}
}

func TestPickerInvalidFiltersWithoutFilesystem(t *testing.T) {
	_, err := RunDerivedPickerContext(context.Background(), io.Discard, strings.NewReader(""), internal.Target{Path: "/missing"}, internal.ScanOptions{Project: "["})
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestPickerNoMatchingEntries(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "other"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := RunDerivedPickerContext(context.Background(), io.Discard, strings.NewReader(""), internal.Target{Path: path}, internal.ScanOptions{Project: "^missing$"})
	if !errors.Is(err, ErrNoEntries) {
		t.Fatal(err)
	}
}
