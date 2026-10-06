package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Finding struct{ Name, State, Detail, Advice string }

type doctorDeps struct {
	run    func(context.Context, string, ...string) (string, error)
	getenv func(string) string
	access func(string) error
	space  func(string) (uint64, error)
}

// Doctor runs informational tools and directory access checks. It never probes
// permissions by writing files, accepts licenses, installs SDKs, or deletes data.
func Doctor(ctx context.Context, targets []Target) ([]Finding, error) {
	return doctor(ctx, targets, doctorDeps{run: doctorCommand, getenv: os.Getenv, access: directoryAccess, space: availableSpace})
}
func doctorCommand(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.WaitDelay = time.Second
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return strings.TrimSpace(string(output)), ctx.Err()
	}
	return strings.TrimSpace(string(output)), err
}
func doctor(ctx context.Context, targets []Target, deps doctorDeps) ([]Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var findings []Finding
	var failures []error
	add := func(f Finding) {
		findings = append(findings, f)
		if f.State == "ERROR" {
			failures = append(failures, fmt.Errorf("%s: %s", f.Name, f.Detail))
		}
	}
	tool := func(label, name, advice string, args ...string) (string, bool) {
		if ctx.Err() != nil {
			return "", false
		}
		output, err := deps.run(ctx, name, args...)
		if err != nil {
			detail := err.Error()
			if output != "" {
				detail += "; " + output
			}
			add(Finding{label, "ERROR", detail, advice})
			return "", false
		}
		if output == "" {
			add(Finding{label, "ERROR", "tool returned empty output", advice})
			return "", false
		}
		return output, true
	}
	override := deps.getenv("DEVELOPER_DIR")
	selection, selectionErr := deps.run(ctx, "xcode-select", "-p")
	if ctx.Err() != nil {
		return findings, ctx.Err()
	}
	ok := selectionErr == nil && selection != ""
	if !ok {
		state := "ERROR"
		if override != "" {
			state = "WARN"
		}
		detail := "No global developer directory selected"
		if selectionErr != nil {
			detail += ": " + selectionErr.Error()
		}
		add(Finding{"Xcode selection", state, detail, "Select full Xcode in Settings > Locations > Command Line Tools."})
	}
	if override != "" {
		add(Finding{"DEVELOPER_DIR", "INFO", override, "This environment override controls developer tools for this shell; unset it to use xcode-select."})
		selection = override
		ok = true
	}

	if ok {
		if strings.Contains(selection, "CommandLineTools") {
			add(Finding{"Developer directory", "ERROR", selection + " selects Command Line Tools rather than full Xcode", "Select full Xcode in Settings > Locations > Command Line Tools."})
		} else {
			add(Finding{"Developer directory", "OK", selection, ""})
		}
	}
	if version, ok := tool("Xcode version", "xcodebuild", "Open Xcode to finish setup or review the error above; doctor does not accept licenses.", "-version"); ok {
		add(Finding{"Xcode version", "OK", version, ""})
	}
	if sdks, ok := tool("iOS SDKs", "xcodebuild", "Install the required iOS platform in Xcode Settings > Components.", "-showsdks"); ok {
		iphone := strings.Contains(sdks, "-sdk iphoneos")
		simulator := strings.Contains(sdks, "-sdk iphonesimulator")
		state := "OK"
		advice := ""
		if !iphone || !simulator {
			state = "ERROR"
			advice = "Install the missing iOS platform in Xcode Settings > Components."
		}
		var iosLines []string
		for _, line := range strings.Split(sdks, "\n") {
			if strings.Contains(line, "-sdk iphone") {
				iosLines = append(iosLines, strings.TrimSpace(line))
			}
		}
		add(Finding{"iOS SDKs", state, fmt.Sprintf("device SDK: %t; simulator SDK: %t\n%s", iphone, simulator, strings.Join(iosLines, "\n")), advice})
	}
	if path, ok := tool("simctl", "xcrun", "Verify full Xcode is selected and finish its first-launch setup.", "--find", "simctl"); ok {
		add(Finding{"simctl", "OK", path, ""})
	}
	if output, ok := tool("Simulator runtimes", "xcrun", "Open Xcode Settings > Components and verify simulator runtimes; review the tool error.", "simctl", "list", "runtimes", "--json"); ok {
		var payload struct {
			Runtimes []struct {
				Name        string `json:"name"`
				Identifier  string `json:"identifier"`
				IsAvailable bool   `json:"isAvailable"`
			} `json:"runtimes"`
		}
		if err := json.Unmarshal([]byte(output), &payload); err != nil {
			add(Finding{"Simulator runtimes", "ERROR", "invalid simctl JSON: " + err.Error(), "Verify the selected Xcode installation."})
		} else {
			count := 0
			var names []string
			for _, r := range payload.Runtimes {
				if r.IsAvailable && strings.Contains(r.Identifier, ".iOS-") {
					count++
					names = append(names, r.Name)
				}
			}
			if count == 0 {
				add(Finding{"Simulator runtimes", "WARN", "No available iOS simulator runtimes", "Install an iOS simulator runtime in Xcode Settings > Components."})
			} else {
				add(Finding{"Simulator runtimes", "OK", strings.Join(names, ", "), ""})
			}
		}
	}
	// Disk space is checked per distinct existing ancestor of each configured root,
	// so a custom DerivedData path on another volume receives its own check.
	disks := map[string]bool{}
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return findings, err
		}
		path := ExpandHome(target.Path)
		absolute, err := filepath.Abs(path)
		if err != nil {
			add(Finding{target.Name, "ERROR", err.Error(), "Check the configured cache path."})
			continue
		}
		info, err := os.Stat(absolute)
		switch {
		case os.IsNotExist(err):
			add(Finding{target.Name, "INFO", absolute + " (not present)", "Missing cache directories are normal; no action is needed."})
		case err != nil:
			add(Finding{target.Name, "ERROR", err.Error(), "Review directory ownership and permissions; doctor makes no changes."})
		case !info.IsDir():
			add(Finding{target.Name, "ERROR", absolute + " is not a directory", "Check the configured cache path."})
		default:
			accessErr := deps.access(absolute)
			if accessErr == nil {
				var file *os.File
				file, accessErr = os.Open(absolute)
				if accessErr == nil {
					_, accessErr = file.ReadDir(1)
					file.Close()
					if accessErr == io.EOF {
						accessErr = nil
					}
				}
			}
			if accessErr != nil {
				add(Finding{target.Name, "ERROR", absolute + ": " + accessErr.Error(), "Review read/write/search permissions and ownership; nested entries may have separate permissions."})
			} else {
				add(Finding{target.Name, "OK", absolute + " (directory read/write/search access)", ""})
			}
		}
		ancestor := absolute
		for {
			info, err := os.Stat(ancestor)
			if err == nil && info.IsDir() {
				break
			}
			if err != nil && !os.IsNotExist(err) {
				break
			}
			parent := filepath.Dir(ancestor)
			if parent == ancestor {
				break
			}
			ancestor = parent
		}
		if disks[ancestor] {
			continue
		}
		disks[ancestor] = true
		free, err := deps.space(ancestor)
		label := "Disk space for " + target.Name
		if err != nil {
			add(Finding{label, "ERROR", err.Error(), "Check that the cache volume is mounted and accessible."})
			continue
		}
		state, advice := "OK", ""
		if free < 10*1024*1024*1024 {
			state = "WARN"
			advice = "Less than 10 GiB is available. Review nuke status before selecting caches to clean."
		}
		add(Finding{label, state, HumanSize(int64(free)) + " available at " + ancestor, advice})
	}
	if err := ctx.Err(); err != nil {
		return findings, err
	}
	return findings, errors.Join(failures...)
}
