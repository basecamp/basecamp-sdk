//go:build windows

package eventfeed

import "errors"

// mkfifoForTest has no FIFO to make on Windows; the subtest that calls it
// skips on this error.
func mkfifoForTest(string) error {
	return errors.New("named pipes are not created by this test on windows")
}
