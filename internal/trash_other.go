//go:build !darwin

package internal

import (
	"errors"
	"os"
)

const trashSupported = false

func moveToTrash(source, trash *os.Root, name string) error {
	return errors.New("moving to Trash is supported only on macOS")
}
