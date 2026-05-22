package tui

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faizmokh/nuke/internal"
)

type scanFinishedMsg struct {
	err error
}

var ErrNoEntries = errors.New("no derived entries matched the interactive selection")

func RunDerivedPicker(out io.Writer, in io.Reader, target internal.Target, projectPattern string, olderThan string) ([]internal.DerivedEntry, error) {
	model := NewDerivedModel(target)

	threshold, err := parseThreshold(olderThan)
	if err != nil {
		return nil, err
	}

	matcher, err := buildProjectMatcher(projectPattern)
	if err != nil {
		return nil, err
	}

	program := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out))
	go func() {
		err := internal.ScanDerivedProgressively(target, func(update internal.DerivedScanUpdate) {
			if !matcher(update.Entry.Name) {
				return
			}
			if threshold != nil && update.Complete && !update.Entry.LastActivity.Before(*threshold) {
				return
			}
			if threshold != nil && !update.Complete {
				return
			}
			program.Send(update)
		})
		program.Send(scanFinishedMsg{err: err})
	}()

	finalModel, err := program.Run()
	if err != nil {
		return nil, err
	}

	derivedModel, ok := finalModel.(*DerivedModel)
	if !ok {
		return nil, fmt.Errorf("unexpected derived picker model type %T", finalModel)
	}
	if derivedModel.scanErr != nil {
		return nil, derivedModel.scanErr
	}
	if derivedModel.scanFinished && !derivedModel.cancelled && len(derivedModel.rows) == 0 {
		return nil, ErrNoEntries
	}

	return derivedModel.Selection(), nil
}

func buildProjectMatcher(projectPattern string) (func(string) bool, error) {
	if projectPattern == "" {
		return func(string) bool { return true }, nil
	}

	re, err := regexp.Compile(projectPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid project pattern: %w", err)
	}
	return re.MatchString, nil
}

func parseThreshold(olderThan string) (*time.Time, error) {
	if olderThan == "" {
		return nil, nil
	}

	threshold, err := internal.ParseAgeThreshold(olderThan)
	if err != nil {
		return nil, err
	}
	return &threshold, nil
}
