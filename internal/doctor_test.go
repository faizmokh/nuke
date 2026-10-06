package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func healthyDoctorDeps() doctorDeps {
	return doctorDeps{
		run: func(ctx context.Context, name string, args ...string) (string, error) {
			key := name + " " + strings.Join(args, " ")
			switch key {
			case "xcode-select -p":
				return "/Applications/Xcode.app/Contents/Developer", nil
			case "xcodebuild -version":
				return "Xcode 26.0\nBuild version test", nil
			case "xcodebuild -showsdks":
				return "iOS -sdk iphoneos26.0\niOS Simulator -sdk iphonesimulator26.0", nil
			case "xcrun --find simctl":
				return "/Applications/Xcode.app/Contents/Developer/usr/bin/simctl", nil
			case "xcrun simctl list runtimes --json":
				return `{"runtimes":[{"name":"iOS 26.0","identifier":"com.apple.CoreSimulator.SimRuntime.iOS-26-0","isAvailable":true}]}`, nil
			default:
				return "", errors.New("unexpected or mutating command: " + key)
			}
		},
		getenv: func(string) string { return "" }, access: func(string) error { return nil }, space: func(string) (uint64, error) { return 30 * 1024 * 1024 * 1024, nil },
	}
}
func containsFinding(findings []Finding, name, state string) bool {
	for _, f := range findings {
		if f.Name == name && f.State == state {
			return true
		}
	}
	return false
}

func TestDoctorHealthyAndMissingCaches(t *testing.T) {
	target := Target{Name: "DerivedData", Path: t.TempDir()}
	missing := Target{Name: "SPM caches", Path: filepath.Join(target.Path, "missing")}
	findings, err := doctor(context.Background(), []Target{target, missing}, healthyDoctorDeps())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Developer directory", "Xcode version", "iOS SDKs", "simctl", "Simulator runtimes", "DerivedData"} {
		if !containsFinding(findings, name, "OK") {
			t.Fatal("missing OK", name, findings)
		}
	}
	if !containsFinding(findings, "SPM caches", "INFO") {
		t.Fatal(findings)
	}
	if _, err := os.Stat(missing.Path); !os.IsNotExist(err) {
		t.Fatal("doctor created cache", err)
	}
}
func TestDoctorWarningsDoNotFail(t *testing.T) {
	deps := healthyDoctorDeps()
	run := deps.run
	deps.run = func(ctx context.Context, name string, args ...string) (string, error) {
		if strings.Join(args, " ") == "simctl list runtimes --json" {
			return `{"runtimes":[]}`, nil
		}
		return run(ctx, name, args...)
	}
	deps.space = func(string) (uint64, error) { return 1024, nil }
	findings, err := doctor(context.Background(), []Target{{Name: "cache", Path: t.TempDir()}}, deps)
	if err != nil || !containsFinding(findings, "Simulator runtimes", "WARN") || !containsFinding(findings, "Disk space for cache", "WARN") {
		t.Fatal(findings, err)
	}
}
func TestDoctorAggregatesToolAndAccessFailures(t *testing.T) {
	deps := healthyDoctorDeps()
	run := deps.run
	deps.run = func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "xcode-select" {
			return "/Library/Developer/CommandLineTools", nil
		}
		if name == "xcodebuild" && args[0] == "-showsdks" {
			return "macOS -sdk macosx", nil
		}
		if name == "xcrun" && args[0] == "simctl" {
			return "malformed", nil
		}
		if name == "xcodebuild" && args[0] == "-version" {
			return "license not accepted", errors.New("exit status 69")
		}
		return run(ctx, name, args...)
	}
	deps.access = func(string) error { return os.ErrPermission }
	findings, err := doctor(context.Background(), []Target{{Name: "cache", Path: t.TempDir()}}, deps)
	if err == nil {
		t.Fatal("failures ignored")
	}
	for _, name := range []string{"Developer directory", "Xcode version", "iOS SDKs", "Simulator runtimes", "cache"} {
		if !containsFinding(findings, name, "ERROR") || !strings.Contains(err.Error(), name) {
			t.Fatal("missing error", name, findings, err)
		}
	}
	if !strings.Contains(err.Error(), "license not accepted") {
		t.Fatal(err)
	}
}
func TestDoctorEnvironmentOverrideControlsSelection(t *testing.T) {
	deps := healthyDoctorDeps()
	run := deps.run
	deps.getenv = func(string) string { return "/Applications/Xcode-Beta.app/Contents/Developer" }
	deps.run = func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "xcode-select" {
			return "/Library/Developer/CommandLineTools", nil
		}
		return run(ctx, name, args...)
	}
	findings, err := doctor(context.Background(), nil, deps)
	if err != nil || !containsFinding(findings, "DEVELOPER_DIR", "INFO") || !containsFinding(findings, "Developer directory", "OK") {
		t.Fatal(findings, err)
	}
}
func TestDoctorCancellationSkipsTools(t *testing.T) {
	deps := healthyDoctorDeps()
	deps.run = func(context.Context, string, ...string) (string, error) {
		t.Fatal("tool ran after cancellation")
		return "", nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := doctor(ctx, nil, deps); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestDoctorInvalidCachePathAndSpaceFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	deps := healthyDoctorDeps()
	deps.space = func(string) (uint64, error) { return 0, errors.New("disk unavailable") }
	findings, err := doctor(context.Background(), []Target{{Name: "cache", Path: path}}, deps)
	if err == nil || !containsFinding(findings, "cache", "ERROR") || !containsFinding(findings, "Disk space for cache", "ERROR") {
		t.Fatal(findings, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatal("file changed", err)
	}
}
func TestAvailableSpaceAndDirectoryAccess(t *testing.T) {
	path := t.TempDir()
	if err := directoryAccess(path); err != nil {
		t.Fatal(err)
	}
	free, err := availableSpace(path)
	if err != nil || free == 0 {
		t.Fatal(free, err)
	}
}

func TestDoctorCommandCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := doctorCommand(ctx, "/bin/sh", "-c", "exec /bin/sleep 30"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
func TestDoctorValidOverrideWithoutGlobalSelection(t *testing.T) {
	deps := healthyDoctorDeps()
	run := deps.run
	deps.getenv = func(string) string { return "/Applications/Xcode.app/Contents/Developer" }
	deps.run = func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "xcode-select" {
			return "", errors.New("no global selection")
		}
		return run(ctx, name, args...)
	}
	findings, err := doctor(context.Background(), nil, deps)
	if err != nil || !containsFinding(findings, "Xcode selection", "WARN") || !containsFinding(findings, "Developer directory", "OK") {
		t.Fatal(findings, err)
	}
}
