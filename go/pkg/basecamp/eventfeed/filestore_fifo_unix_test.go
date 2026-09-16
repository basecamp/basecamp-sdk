//go:build unix

package eventfeed

import "syscall"

// mkfifoForTest creates a named pipe at path for the not-a-regular-file
// refusal test. Unix only: the Windows build has no FIFO to make, and the
// subtest skips on the error the other file returns.
func mkfifoForTest(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
