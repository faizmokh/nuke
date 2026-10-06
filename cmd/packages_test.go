package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizmokh/nuke/internal/tui"
)

func cmdPackageFixture(t *testing.T, root, kind, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	marker := map[string]string{"project": "project.pbxproj", "workspace": "contents.xcworkspacedata", "package": "Package.swift"}[kind]
	if err := os.WriteFile(filepath.Join(path, marker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func TestSPMDownloadLockedAndInitial(t *testing.T) {
	original := runPackageTool
	t.Cleanup(func() { runPackageTool = original })
	for _, kind := range []string{"project", "workspace", "package"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			name := map[string]string{"project": "App.xcodeproj", "workspace": "App.xcworkspace", "package": "Core"}[kind]
			path := cmdPackageFixture(t, root, kind, name)
			resolved := filepath.Join(path, "Package.resolved")
			if kind == "project" {
				resolved = filepath.Join(path, "project.xcworkspace", "xcshareddata", "swiftpm", "Package.resolved")
			}
			if kind == "workspace" {
				resolved = filepath.Join(path, "xcshareddata", "swiftpm", "Package.resolved")
			}
			for _, locked := range []bool{false, true} {
				if locked {
					os.MkdirAll(filepath.Dir(resolved), 0755)
					os.WriteFile(resolved, []byte("{}"), 0600)
				}
				calls := 0
				runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
					if len(args) > 0 && args[0] == "-list" {
						fmt.Fprintln(out, `{"workspace":{"schemes":["App"]}}`)
						return nil
					}
					calls++
					if calls == 1 {
						return nil
					}
					joined := strings.Join(args, " ")
					hasLock := strings.Contains(joined, "resolved") || strings.Contains(joined, "Resolved")
					if hasLock != locked {
						t.Fatalf("locked=%v args=%v", locked, args)
					}
					fmt.Fprintln(out, "Fetching https://user:secret@example.com/Repo.git")
					fmt.Fprintln(out, "Resolved source packages:")
					return nil
				}
				cmd := NewRootCommand()
				cmd.SetIn(forbiddenInput{})
				out, err := executeCommand(cmd, "spm", "download", "--"+kind, path)
				if err != nil || calls != 2 || !strings.Contains(out, "Dependencies ready") || strings.Contains(out, "secret") {
					t.Fatalf("%s %v calls=%d", out, err, calls)
				}
			}
		})
	}
}
func TestSPMNoPromptAndPicker(t *testing.T) {
	terminal, choice, runner := isInteractiveTerminal, runPackageChoice, runPackageTool
	t.Cleanup(func() { isInteractiveTerminal = terminal; runPackageChoice = choice; runPackageTool = runner })
	root := t.TempDir()
	cmdPackageFixture(t, root, "workspace", "One.xcworkspace")
	second := cmdPackageFixture(t, root, "workspace", "Two.xcworkspace")
	t.Chdir(root)
	cmd := NewRootCommand()
	cmd.SetIn(forbiddenInput{})
	if _, err := executeCommand(cmd, "spm", "download"); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatal(err)
	}
	isInteractiveTerminal = func(io.Reader, io.Writer) bool { return true }
	runPackageChoice = func(ctx context.Context, out io.Writer, in io.Reader, title string, choices []tui.TargetChoice) (string, error) {
		if len(choices) != 2 || !strings.Contains(title, "package") {
			t.Fatal(choices, title)
		}
		return second, nil
	}
	runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
		if len(args) > 0 && args[0] == "-list" {
			fmt.Fprintln(out, `{"workspace":{"schemes":["App"]}}`)
			return nil
		}
		if len(args) > 1 && !strings.Contains(strings.Join(args, " "), second) {
			t.Fatal(args)
		}
		return nil
	}
	if out, err := executeCommand(NewRootCommand(), "spm", "download"); err != nil || !strings.Contains(out, second) {
		t.Fatalf("%s %v", out, err)
	}
	runPackageChoice = func(context.Context, io.Writer, io.Reader, string, []tui.TargetChoice) (string, error) {
		return "", nil
	}
	runPackageTool = func(context.Context, io.Writer, string, ...string) error {
		t.Fatal("ran after cancellation")
		return nil
	}
	if _, err := executeCommand(NewRootCommand(), "spm", "download"); err != nil {
		t.Fatal(err)
	}
}
func TestSPMRejectsInvalidFlagsBeforeTools(t *testing.T) {
	original := runPackageTool
	t.Cleanup(func() { runPackageTool = original })
	runPackageTool = func(context.Context, io.Writer, string, ...string) error { t.Fatal("tool invoked"); return nil }
	root := t.TempDir()
	pkg := cmdPackageFixture(t, root, "package", "Core")
	for _, args := range [][]string{{"--package", pkg, "--scheme", "App"}, {"--package", pkg, "--derived-data", root}, {"--project", ""}, {"--scheme", ""}, {"--project", "A", "--workspace", "B"}, {"extra"}} {
		if _, err := executeCommand(NewRootCommand(), append([]string{"spm", "download"}, args...)...); err == nil {
			t.Fatal(args)
		}
	}
}
func TestSPMFailureAndCancellation(t *testing.T) {
	original := runPackageTool
	t.Cleanup(func() { runPackageTool = original })
	pkg := cmdPackageFixture(t, t.TempDir(), "package", "Core")
	calls := 0
	runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
		calls++
		if calls == 1 {
			return nil
		}
		fmt.Fprintln(out, "fatal: Authentication failed for https://user:password@example.com/repo")
		return errors.New("exit status 1")
	}
	out, err := executeCommand(NewRootCommand(), "spm", "download", "--package", pkg)
	if err == nil || calls != 2 || !strings.Contains(err.Error(), "credentials") || strings.Contains(out, "user:password") {
		t.Fatalf("%s %v %d", out, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := NewRootCommand()
	cmd.SetContext(ctx)
	if _, err := executeCommand(cmd, "spm", "download", "--package", pkg); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestPackageOutputChunksAndRedaction(t *testing.T) {
	var out bytes.Buffer
	p := newPackageOutput(&out, true)
	for _, chunk := range []string{"Fetching https://u:sec", "ret@example.com/repo?token=abc\r", "\x1b[31merror: failed\x1b[0m\n", "Authorization: Bearer hidden\n", "last line"} {
		if _, err := p.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	p.flush()
	text := out.String()
	for _, secret := range []string{"secret", "token=abc", "hidden", "\x1b"} {
		if strings.Contains(text, secret) {
			t.Fatal(text)
		}
	}
	if !strings.Contains(text, "last line") || !strings.Contains(text, "[redacted]") {
		t.Fatal(text)
	}
	for i := 0; i < 100; i++ {
		fmt.Fprintln(p, "line")
	}
	if len(p.tail) != 20 {
		t.Fatal(len(p.tail))
	}
}
func TestPackageRetryClassification(t *testing.T) {
	for _, s := range []string{"Authentication failed; connection reset", "checksum mismatch", "Package.resolved out of date", "permission denied", "version conflict"} {
		if transientPackageFailure(s) {
			t.Fatal(s)
		}
	}
	if !transientPackageFailure("fatal: connection reset by peer") {
		t.Fatal("transient missed")
	}
}

func TestSPMRetryAndCancellationDuringRetry(t *testing.T) {
	original := runPackageTool
	t.Cleanup(func() { runPackageTool = original })
	pkg := cmdPackageFixture(t, t.TempDir(), "package", "Core")
	calls := 0
	runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
		calls++
		if calls == 2 {
			fmt.Fprintln(out, "connection reset by peer")
			return errors.New("network failure")
		}
		return nil
	}
	out, err := executeCommand(NewRootCommand(), "spm", "download", "--package", pkg)
	if err != nil || calls != 3 || !strings.Contains(out, "retrying (1/2)") {
		t.Fatalf("%s %v %d", out, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls = 0
	runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
		calls++
		if calls == 1 {
			return nil
		}
		fmt.Fprintln(out, "connection reset by peer")
		cancel()
		return ctx.Err()
	}
	cmd := NewRootCommand()
	cmd.SetContext(ctx)
	out, err = executeCommand(cmd, "spm", "download", "--package", pkg)
	if !errors.Is(err, context.Canceled) || calls != 2 || !strings.Contains(out, "cancelled") {
		t.Fatalf("%s %v %d", out, err, calls)
	}
}

func TestPackageSchemeChoices(t *testing.T) {
	originalRunner, originalTerminal, originalChoice := runPackageTool, isInteractiveTerminal, runPackageChoice
	t.Cleanup(func() {
		runPackageTool = originalRunner
		isInteractiveTerminal = originalTerminal
		runPackageChoice = originalChoice
	})
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetIn(forbiddenInput{})
	cmd.SetContext(context.Background())
	output := newPackageOutput(&out, false)
	for _, test := range []struct {
		json, want string
		failure    bool
	}{{`{"workspace":{"schemes":["App"]}}`, "App", false}, {`{"workspace":{"schemes":[]}}`, "", true}, {`{"workspace":{"schemes":["B","A"]}}`, "", true}, {`invalid`, "", true}} {
		runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
			if !strings.Contains(strings.Join(args, " "), "-disableAutomaticPackageResolution") {
				t.Fatal(args)
			}
			fmt.Fprintln(out, test.json)
			return nil
		}
		got, err := choosePackageScheme(cmd, "workspace", "/tmp/App.xcworkspace", output)
		if got != test.want || (err != nil) != test.failure {
			t.Fatalf("%s %s %v", test.json, got, err)
		}
	}
	runPackageTool = func(ctx context.Context, out io.Writer, tool string, args ...string) error {
		fmt.Fprintln(out, `{"workspace":{"schemes":["B","A"]}}`)
		return nil
	}
	isInteractiveTerminal = func(io.Reader, io.Writer) bool { return true }
	runPackageChoice = func(ctx context.Context, out io.Writer, in io.Reader, title string, choices []tui.TargetChoice) (string, error) {
		if choices[0].Name != "A" {
			t.Fatal(choices)
		}
		return "B", nil
	}
	if got, err := choosePackageScheme(cmd, "workspace", "/tmp/App.xcworkspace", output); got != "B" || err != nil {
		t.Fatal(got, err)
	}
	runPackageChoice = func(context.Context, io.Writer, io.Reader, string, []tui.TargetChoice) (string, error) {
		return "", nil
	}
	if _, err := choosePackageScheme(cmd, "workspace", "/tmp/App.xcworkspace", output); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
