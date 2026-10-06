//go:build darwin

package internal

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

const trashSupported = true

func moveToTrash(source, trash *os.Root, name string) error {
	from, err := source.Open(".")
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := trash.Open(".")
	if err != nil {
		return err
	}
	defer to.Close()
	destination := name
	for attempt := 0; attempt < 10; attempt++ {
		// Anchor both paths at opened directories and never overwrite an existing
		// Trash item, even if another process creates it concurrently.
		err = unix.RenameatxNp(int(from.Fd()), name, int(to.Fd()), destination, unix.RENAME_EXCL)
		if !errors.Is(err, unix.EEXIST) {
			return err
		}
		destination = fmt.Sprintf("%s-nuke-%s", name, rand.Text())
	}
	return fmt.Errorf("could not choose an unused Trash name: %w", err)
}
