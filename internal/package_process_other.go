//go:build !unix

package internal

import (
	"fmt"
	"os/exec"
)

func configurePackageProcess(cmd *exec.Cmd) {}
func lockPackageFile(dir, path string) (func(), error) {
	return nil, fmt.Errorf("package downloads require macOS or a Unix host")
}
