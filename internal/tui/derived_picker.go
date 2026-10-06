package tui

import (
	"context"
	"errors"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faizmokh/nuke/internal"
)

type scanFinishedMsg struct {
	plan internal.ScanPlan
	err  error
}

var ErrNoEntries = errors.New("no derived entries matched the interactive selection")

func RunDerivedPickerContext(ctx context.Context, out io.Writer, in io.Reader, target internal.Target, options internal.ScanOptions) (internal.ScanPlan, error) {
	return runDerivedPicker(ctx, out, in, target, options, internal.ScanTarget)
}

func runDerivedPicker(ctx context.Context, out io.Writer, in io.Reader, target internal.Target, options internal.ScanOptions, scan func(context.Context, internal.Target, internal.ScanOptions, func(internal.ScanUpdate)) (internal.ScanPlan, error)) (internal.ScanPlan, error) {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	model := NewDerivedModel(target)
	program := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithContext(ctx), tea.WithFPS(10))
	done := make(chan struct{})
	go func() {
		defer close(done)
		plan, err := scan(scanCtx, target, options, func(u internal.ScanUpdate) { program.Send(u) })
		program.Send(scanFinishedMsg{plan: plan, err: err})
	}()
	final, err := program.Run()
	cancel()
	<-done // Don't leave a scan running after the UI exits.
	if ctx.Err() != nil {
		return internal.ScanPlan{}, ctx.Err()
	}
	if err != nil {
		return internal.ScanPlan{}, err
	}
	m, ok := final.(*DerivedModel)
	if !ok {
		return internal.ScanPlan{}, fmt.Errorf("unexpected derived picker model type %T", final)
	}
	if m.cancelled {
		return internal.ScanPlan{}, nil
	}
	if m.scanFinished && len(m.rows) == 0 && m.scanErr == nil {
		return m.plan, ErrNoEntries
	}
	return m.plan.Select(m.Selection()), m.scanErr
}

func RunDerivedPicker(out io.Writer, in io.Reader, target internal.Target, projectPattern, olderThan string) ([]internal.DerivedEntry, error) {
	plan, err := RunDerivedPickerContext(context.Background(), out, in, target, internal.ScanOptions{Project: projectPattern, OlderThan: olderThan, Activity: true})
	return plan.Entries, err
}
