package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var runPackageTool = internal.RunPackageTool
var runPackageChoice = tui.RunChoiceMenu

func newSPMCommand() *cobra.Command {
	group := &cobra.Command{Use: "spm", Short: "Prepare Swift package dependencies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("choose a package operation: nuke spm download (see nuke spm --help)")
	}}
	var project, workspace, pkg, scheme string
	var verbose bool
	download := &cobra.Command{Use: "download", Short: "Resolve and download existing project dependencies", Args: cobra.NoArgs}
	download.Flags().StringVar(&project, "project", "", "path to an Xcode .xcodeproj")
	download.Flags().StringVar(&workspace, "workspace", "", "path to an Xcode .xcworkspace")
	download.Flags().StringVar(&pkg, "package", "", "path to a standalone Swift package directory or Package.swift")
	download.Flags().StringVar(&scheme, "scheme", "", "resolve dependencies for a specific Xcode scheme")
	download.Flags().BoolVar(&verbose, "verbose", false, "show full tool output, with credentials redacted")
	download.MarkFlagsMutuallyExclusive("project", "workspace", "package")
	download.Example = "  nuke spm download\n  nuke spm download --workspace App.xcworkspace\n  nuke spm download --project App.xcodeproj --scheme App\n  nuke spm download --package Packages/Core"
	download.RunE = func(cmd *cobra.Command, args []string) error {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		for _, flag := range []string{"project", "workspace", "package", "scheme", "derived-data"} {
			if cmd.Flags().Changed(flag) {
				value, _ := cmd.Flags().GetString(flag)
				if strings.TrimSpace(value) == "" {
					return fmt.Errorf("--%s requires a nonempty value", flag)
				}
			}
		}
		var setups []internal.PackageSetup
		var err error
		for _, explicit := range []struct{ kind, path string }{{"project", project}, {"workspace", workspace}, {"package", pkg}} {
			if explicit.path != "" {
				var setup internal.PackageSetup
				setup, err = internal.ValidatePackageSetup(explicit.kind, explicit.path)
				setups = []internal.PackageSetup{setup}
				break
			}
		}
		if len(setups) == 0 && err == nil {
			var cwd string
			cwd, err = os.Getwd()
			if err == nil {
				setups, err = internal.DiscoverPackageSetups(cmd.Context(), cwd)
			}
		}
		if err != nil {
			return err
		}
		setup := setups[0]
		if len(setups) > 1 {
			if !isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
				var paths []string
				for _, s := range setups {
					paths = append(paths, s.Path)
				}
				return fmt.Errorf("multiple %ss found: %s; specify --workspace or --project", setup.Kind, strings.Join(paths, ", "))
			}
			var choices []tui.TargetChoice
			for _, s := range setups {
				choices = append(choices, tui.TargetChoice{Name: s.Path, Label: filepath.Base(s.Path), Description: s.Path})
			}
			selected, err := runPackageChoice(cmd.Context(), cmd.OutOrStdout(), cmd.InOrStdin(), "Choose a project for package downloads", choices)
			if err != nil {
				return err
			}
			if selected == "" {
				cmd.Println("Download cancelled.")
				return nil
			}
			found := false
			for _, s := range setups {
				if s.Path == selected {
					setup = s
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid project selection")
			}
		}
		derived, _ := cmd.Flags().GetString("derived-data")
		if setup.Kind == "package" && (scheme != "" || derived != "") {
			return fmt.Errorf("--scheme and --derived-data apply only to Xcode projects/workspaces")
		}
		unlock, err := internal.LockPackageSetup(setup.Path)
		if err != nil {
			return err
		}
		defer unlock()
		info, err := os.Stat(setup.ResolvedPath())
		locked := err == nil
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read Package.resolved: %w", err)
		}
		if locked && !info.Mode().IsRegular() {
			return fmt.Errorf("Package.resolved must be a regular file")
		}
		output := newPackageOutput(cmd.OutOrStdout(), verbose)
		output.message(fmt.Sprintf("%s: %s", strings.ToUpper(setup.Kind[:1])+setup.Kind[1:], setup.Path))
		if scheme != "" {
			output.message("Scheme: " + scheme)
		}
		if locked {
			output.message("Using locked versions from Package.resolved.")
		} else {
			output.message("No Package.resolved: resolving initial versions from project requirements (may create a lockfile).")
		}
		name, _ := setup.DownloadArgs(scheme, derived, locked, verbose)
		// Preflight is read-only; it never accepts licenses or runs first-launch setup.
		output.message("Checking developer tools…")
		checkCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		checkArgs := []string{"--version"}
		if name == "xcodebuild" {
			checkArgs = []string{"-version"}
		}
		err = runPackageTool(checkCtx, output, name, checkArgs...)
		cancel()
		output.flush()
		if err != nil {
			if cmd.Context().Err() != nil {
				return cmd.Context().Err()
			}
			return fmt.Errorf("developer tools unavailable: %w; check xcode-select, install Xcode, and complete its first-launch setup", err)
		}
		if scheme == "" && (setup.Kind == "workspace" || derived != "") {
			output.message("Finding Xcode schemes…")
			scheme, err = choosePackageScheme(cmd, setup.Kind, setup.Path, output)
			if err != nil {
				return err
			}
			output.message("Scheme: " + scheme)
		}
		_, toolArgs := setup.DownloadArgs(scheme, derived, locked, verbose)
		output.resetTail()
		output.message("Resolving and downloading dependencies…")
		started := time.Now()
		done := make(chan struct{})
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					output.heartbeat(time.Since(started))
				}
			}
		}()
		for attempt := 0; attempt < 3; attempt++ {
			err = runPackageTool(cmd.Context(), output, name, toolArgs...)
			output.flush()
			if err == nil || cmd.Context().Err() != nil || attempt == 2 || !transientPackageFailure(output.tailText()) {
				break
			}
			output.message(fmt.Sprintf("Temporary network failure; retrying (%d/2)…", attempt+1))
			timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
			select {
			case <-cmd.Context().Done():
				timer.Stop()
				err = cmd.Context().Err()
			case <-timer.C:
				output.resetTail()
			}
			if cmd.Context().Err() != nil {
				break
			}
		}
		close(done)
		<-stopped
		if cmd.Context().Err() != nil {
			output.message("Download cancelled.")
			return cmd.Context().Err()
		}
		if err != nil {
			output.message("Download failed. Recent tool output:")
			for _, line := range strings.Split(output.tailText(), "\n") {
				output.message("  " + line)
			}
			return fmt.Errorf("%w; %s", err, packageFailureHint(output.tailText()))
		}
		output.message(fmt.Sprintf("✓ Dependencies ready · %s", time.Since(started).Round(time.Second)))
		return output.errValue()
	}
	group.AddCommand(download)
	return group
}

// Xcode requires a scheme for workspaces and custom DerivedData paths.
// Disable automatic package resolution while inspecting available schemes.
func choosePackageScheme(cmd *cobra.Command, kind, path string, output *packageOutput) (string, error) {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	var raw bytes.Buffer
	capture := &boundedPackageCapture{buffer: &raw, remaining: 4 * 1024 * 1024}
	err := runPackageTool(ctx, capture, "xcodebuild", "-list", "-json", "-"+kind, path, "-disableAutomaticPackageResolution")
	if err != nil {
		output.message(raw.String())
		return "", fmt.Errorf("cannot discover schemes: %w; specify --scheme explicitly", err)
	}
	data := bytes.TrimSpace(raw.Bytes())
	// Some Xcode versions prefix JSON with informational lines.
	if idx := bytes.IndexByte(data, '{'); idx >= 0 {
		data = data[idx:]
	}
	var payload struct {
		Project   struct{ Schemes []string }
		Workspace struct{ Schemes []string }
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("cannot read Xcode schemes; specify --scheme explicitly: %w", err)
	}
	schemes := payload.Project.Schemes
	if kind == "workspace" {
		schemes = payload.Workspace.Schemes
	}
	sort.Strings(schemes)
	schemes = compactSchemes(schemes)
	if len(schemes) == 0 {
		return "", fmt.Errorf("no schemes available in %s; create or share a scheme in Xcode, then retry", path)
	}
	if len(schemes) == 1 {
		return schemes[0], nil
	}
	if !isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
		return "", fmt.Errorf("multiple schemes found: %s; specify --scheme", strings.Join(schemes, ", "))
	}
	var choices []tui.TargetChoice
	for _, scheme := range schemes {
		choices = append(choices, tui.TargetChoice{Name: scheme, Label: scheme, Description: filepath.Base(path)})
	}
	selected, err := runPackageChoice(cmd.Context(), cmd.OutOrStdout(), cmd.InOrStdin(), "Choose a scheme for package downloads", choices)
	if err != nil {
		return "", err
	}
	if selected == "" {
		return "", context.Canceled
	}
	for _, scheme := range schemes {
		if selected == scheme {
			return selected, nil
		}
	}
	return "", fmt.Errorf("invalid scheme selection")
}
func compactSchemes(schemes []string) []string {
	var result []string
	for _, scheme := range schemes {
		if scheme != "" && (len(result) == 0 || result[len(result)-1] != scheme) {
			result = append(result, scheme)
		}
	}
	return result
}

type boundedPackageCapture struct {
	buffer    *bytes.Buffer
	remaining int
}

func (c *boundedPackageCapture) Write(p []byte) (int, error) {
	if len(p) > c.remaining {
		return 0, fmt.Errorf("Xcode scheme output exceeded 4 MiB")
	}
	c.remaining -= len(p)
	return c.buffer.Write(p)
}

var credentialURL = regexp.MustCompile(`(?i)(https?://)[^/\s@]+@`)
var secretQuery = regexp.MustCompile(`(?i)([?&](?:token|access_token|auth|key|password|signature|credential|x-amz-signature|x-amz-credential)=)[^\s&#]+`)
var secretAssignment = regexp.MustCompile(`(?i)((?:authorization|password|access_token|token)\s*[:=]\s*)[^\s,;]+(?:\s+[^\s,;]+)?`)

func safePackageText(s string) string {
	s = ansi.Strip(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, s)
	s = credentialURL.ReplaceAllString(s, "${1}[redacted]@")
	s = secretQuery.ReplaceAllString(s, "${1}[redacted]")
	return secretAssignment.ReplaceAllString(s, "${1}[redacted]")
}

type packageOutput struct {
	mu       sync.Mutex
	out      io.Writer
	verbose  bool
	pending  string
	tail     []string
	latest   string
	writeErr error
}

func newPackageOutput(out io.Writer, verbose bool) *packageOutput {
	return &packageOutput{out: out, verbose: verbose}
}
func (o *packageOutput) print(s string) {
	if o.writeErr == nil {
		_, o.writeErr = fmt.Fprintln(o.out, s)
	}
}
func (o *packageOutput) message(s string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.print(safePackageText(s))
}
func (o *packageOutput) line(s string) {
	s = strings.TrimSpace(safePackageText(s))
	if s == "" {
		return
	}
	o.latest = s
	o.tail = append(o.tail, s)
	if len(o.tail) > 20 {
		o.tail = o.tail[len(o.tail)-20:]
	}
	lower := strings.ToLower(s)
	show := o.verbose
	if !o.verbose && strings.Contains(s, " -resolvePackageDependencies ") {
		return
	}
	for _, word := range []string{"fetch", "download", "check", "resolv", "comput", "updat", "error", "warning", "fail", "swift version", "xcode ", "build version", "artifact", "cached", "creating working copy"} {
		if strings.Contains(lower, word) {
			show = true
			break
		}
	}
	if show {
		o.print("  " + s)
	}
}
func (o *packageOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pending += strings.ReplaceAll(string(p), "\r", "\n")
	for {
		idx := strings.IndexByte(o.pending, '\n')
		if idx < 0 {
			break
		}
		o.line(o.pending[:idx])
		o.pending = o.pending[idx+1:]
	}
	if len(o.pending) > 64*1024 {
		o.line(o.pending)
		o.pending = ""
	}
	return len(p), o.writeErr
}
func (o *packageOutput) flush() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pending != "" {
		o.line(o.pending)
		o.pending = ""
	}
}
func (o *packageOutput) heartbeat(elapsed time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	latest := ansi.Truncate(o.latest, 100, "…")
	if latest == "" {
		latest = "waiting for tool output"
	}
	o.print(fmt.Sprintf("  Still working · %s elapsed · %s", elapsed.Round(time.Second), latest))
}
func (o *packageOutput) tailText() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return strings.Join(o.tail, "\n")
}
func (o *packageOutput) resetTail()      { o.mu.Lock(); defer o.mu.Unlock(); o.tail = nil; o.latest = "" }
func (o *packageOutput) errValue() error { o.mu.Lock(); defer o.mu.Unlock(); return o.writeErr }
func transientPackageFailure(s string) bool {
	s = strings.ToLower(s)
	for _, word := range []string{"authentication", "permission denied", "unauthorized", "forbidden", "checksum", "could not resolve dependencies", "version conflict", "package.resolved"} {
		if strings.Contains(s, word) {
			return false
		}
	}
	for _, word := range []string{"connection reset", "connection timed out", "operation timed out", "could not resolve host", "temporary failure", "http 502", "http 503", "http 504"} {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}
func packageFailureHint(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "authentication") || strings.Contains(s, "permission denied") || strings.Contains(s, "could not read username") || strings.Contains(s, "401") || strings.Contains(s, "403"):
		return "check private repository credentials in Xcode, Git/SSH, or netrc; nuke cannot supply credentials"
	case strings.Contains(s, "no space left"):
		return "free disk space, then retry"
	case strings.Contains(s, "checksum") || strings.Contains(s, "signature"):
		return "check the dependency's published artifact; verification remains enabled"
	case strings.Contains(s, "package.resolved"):
		return "resolve the changed requirements in Xcode or SwiftPM, review Package.resolved, then retry"
	case strings.Contains(s, "license") || strings.Contains(s, "first launch"):
		return "open Xcode and complete license/first-launch setup"
	case transientPackageFailure(s):
		return "check your network or proxy, then retry"
	default:
		return "review the tool error above; use --verbose for more detail (local package paths, schemes, permissions, or dependency requirements may need fixing)"
	}
}
