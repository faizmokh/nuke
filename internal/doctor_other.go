//go:build !darwin && !linux

package internal

import "errors"

func directoryAccess(string) error {
	return errors.New("directory access diagnostics require macOS or Linux")
}
func availableSpace(string) (uint64, error) {
	return 0, errors.New("disk space diagnostics require macOS or Linux")
}
