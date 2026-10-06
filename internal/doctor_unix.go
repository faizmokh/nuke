//go:build darwin || linux

package internal

import "golang.org/x/sys/unix"

func directoryAccess(path string) error { return unix.Access(path, unix.R_OK|unix.W_OK|unix.X_OK) }
func availableSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
