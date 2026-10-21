//go:build unix

package input

import (
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// pendingStdin reports whether file holds unread input, without consuming any
// of it and without waiting longer than wait. ok is false when the file type
// cannot be inspected this way; the caller then falls back to reading.
func pendingStdin(file *os.File, wait time.Duration) (pending, ok bool, err error) {
	info, err := file.Stat()
	if err != nil {
		return false, false, err
	}
	mode := info.Mode()
	switch {
	case mode.IsRegular():
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return false, false, err
		}
		return info.Size() > offset, true, nil
	case mode&(os.ModeNamedPipe|os.ModeSocket) != 0:
		// SyscallConn, unlike Fd, leaves the descriptor's blocking mode
		// alone; that mode is shared with the parent process.
		raw, err := file.SyscallConn()
		if err != nil {
			return false, false, err
		}
		var probeErr error
		if err := raw.Control(func(fd uintptr) {
			pending, ok, probeErr = pollPending(int(fd), wait)
		}); err != nil {
			return false, false, err
		}
		return pending, ok, probeErr
	default:
		return false, false, nil
	}
}

// pollPending waits until fd is readable or wait elapses. Input that has not
// arrived by then is treated as absent, so an idle pipe that is never closed
// cannot block the command.
func pollPending(fd int, wait time.Duration) (pending, ok bool, err error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	deadline := time.Now().Add(wait)
	for {
		timeout := max(int(time.Until(deadline)/time.Millisecond), 0)
		ready, err := unix.Poll(fds, timeout)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return false, false, err
		}
		if ready == 0 {
			return false, true, nil
		}
		if fionread == 0 {
			// The descriptor is readable, so a fallback read returns at once.
			return false, false, nil
		}
		available, err := unix.IoctlGetInt(fd, fionread)
		if err != nil {
			return false, false, nil
		}
		return available > 0, true, nil
	}
}
