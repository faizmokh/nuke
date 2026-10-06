package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RunPackageTool streams both output channels. It never connects child stdin:
// authentication must use credentials already available to Xcode/Git/SwiftPM.
func RunPackageTool(ctx context.Context, out io.Writer, name string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "xcrun", append([]string{name}, args...)...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = 2 * time.Second
	configurePackageProcess(cmd)
	err := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

// LockPackageSetup uses a persistent lock file outside the user's repository.
// OS locks are released on cancellation, errors, and process crashes.
func LockPackageSetup(path string) (func(), error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "nuke", "locks")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create download lock directory: %w", err)
	}
	return lockPackageFile(dir, path)
}
