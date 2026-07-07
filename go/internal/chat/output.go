package chat

import (
	"os"
)

// openOutput opens the output file (returning os.Stdout when path is empty).
// The caller is responsible for Close(). When stdout is selected, truncate
// and seek operations are no-ops because we can't rewind the user's
// terminal.
func openOutput(path string) (*os.File, error) {
	if path == "" {
		return os.Stdout, nil
	}
	return os.Create(path)
}

// resetOutput truncates the writer back to length 0 (when possible). When
// the writer is non-truncatable (e.g. stdout), this is a no-op.
func resetOutput(f *os.File) error {
	if f == os.Stdout {
		return nil
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err := f.Seek(0, 0)
	return err
}